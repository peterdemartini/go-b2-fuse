package b2fs

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"path/filepath"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/peterdemartini/go-backblaze"
	De "github.com/tj/go-debug"
)

// Config defines the configuration
type Config struct {
	AccountID      string
	ApplicationKey string
	BucketID       string
	MountPoint     string
}

// DirItem is a list item
type DirItem struct {
	mode     uint32
	isDir    bool
	dir      string
	fullPath string
	fileName string
	file     backblaze.FileStatus
}

// B2FS defines the struct
type B2FS struct {
	pathfs.FileSystem
	config         *Config
	b2             *backblaze.B2
	bucket         *backblaze.Bucket
	dirItems       []DirItem
	fetchingFiles  bool
	fetchingError  error
	filesUpdatedAt int
}

// GetAttr gets file attributes
func (client *B2FS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	var debug = De.Debug("go-b2-fuse:GetAttr")
	name = escapeName(name)
	debug("name: %s", name)
	err := client.getFiles(filepath.Dir(name))
	if err != nil {
		fmt.Printf("Error listing file names %v", err)
		return nil, fuse.ENOENT
	}
	if name == "" {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}
	for _, dirItem := range client.dirItems {
		if dirItem.fullPath == name {
			var mode uint32
			if dirItem.isDir {
				mode = dirItem.mode | 0755
				debug("matched dir %s", dirItem.fullPath)
			} else {
				mode = dirItem.mode | 0644
				debug("matched file %s", dirItem.fullPath)
			}
			return &fuse.Attr{
				Mode: mode,
				Size: uint64(dirItem.file.Size),
			}, fuse.OK
		}
	}
	return nil, fuse.ENOENT
}

// OpenDir will open directory
func (client *B2FS) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	var debug = De.Debug("go-b2-fuse:OpenDir")
	name = escapeName(name)
	debug("name: %s", name)
	list := []fuse.DirEntry{}
	err := client.getFiles(name)
	if err != nil {
		fmt.Printf("Error listing file names %v", err)
		return nil, fuse.ENOENT
	}

	for _, dirItem := range client.dirItems {
		// debug("comparing %s == %s", name, dirItem.dir)
		if name == dirItem.dir {
			debug("found file: %s", dirItem.fullPath)
			list = append(list, fuse.DirEntry{
				Name: dirItem.fileName,
				Mode: dirItem.mode,
			})
		}
	}
	return list, fuse.OK
}

// Open will open a file
func (client *B2FS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	var debug = De.Debug("go-b2-fuse:Open")
	name = escapeName(name)
	err := client.getFiles(filepath.Dir(name))
	if err != nil {
		fmt.Printf("Error listing file names %v", err)
		return nil, fuse.ENOENT
	}
	debug("name: %s", name)
	for _, item := range client.dirItems {
		if item.fullPath == name {
			debug("matched file %s", item.fullPath)
			_, r, err := client.b2.DownloadFileByID(item.file.ID)
			if err != nil {
				fmt.Printf("Error downloading file %v", err.Error())
				return nil, fuse.ENODATA
			}
			defer r.Close()
			bytes, err := ioutil.ReadAll(r)
			if err != nil {
				fmt.Printf("Error reading file %v", err.Error())
				return nil, fuse.ENODATA
			}
			return nodefs.NewDataFile(bytes), fuse.OK
		}
	}
	return nil, fuse.ENOENT
}

func (client *B2FS) getFiles(dir string) error {
	var debug = De.Debug("go-b2-fuse:getFiles")
	if client.fetchingFiles {
		fmt.Println("waiting for files")
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for {
				if !client.fetchingFiles {
					wg.Done()
					return
				}
				time.Sleep(time.Millisecond * 5)
			}
		}()
		wg.Wait()
		debug("done waiting")
		return client.fetchingError
	}

	updated := client.filesUpdatedAt
	if updated > 0 {
		since := getNowMS() - 500
		if (updated - since) > 0 {
			return nil
		}
	}
	client.fetchingError = nil
	client.fetchingFiles = true
	files, err := client.bucket.ListFileNames(dir, 0)
	if err != nil {
		fmt.Printf("error getting files %v", err.Error())
		client.fetchingError = err
		return err
	}
	client.dirItems = []DirItem{}
	for _, file := range files.Files {
		paths, fullFile := getPaths(file.Name)
		client.addDirItem(file, fullFile, false)
		for _, path := range paths {
			client.addDirItem(file, path, true)
		}
	}
	client.fetchingFiles = false
	client.fetchingError = nil
	client.filesUpdatedAt = getNowMS()
	debug("got files %v", len(files.Files))
	return nil
}

func (client *B2FS) addDirItem(file backblaze.FileStatus, fullPath string, isDir bool) {
	var debug = De.Debug("go-b2-fuse:addDirItem")
	if client.dirItemExists(fullPath) {
		return
	}
	dir, fileName := filepath.Split(fullPath)
	var mode uint32
	if isDir {
		mode = fuse.S_IFDIR
	} else {
		mode = fuse.S_IFREG
	}
	dir = escapePath(dir)
	fullPath = escapePath(fullPath)
	fileName = escapePath(fileName)
	dirItem := DirItem{
		mode:     mode,
		isDir:    isDir,
		file:     file,
		fullPath: fullPath,
		fileName: fileName,
		dir:      dir,
	}
	debug("adding dirItem fileName: %s dir: %s fullPath: %s", fileName, dir, fullPath)
	client.dirItems = append(client.dirItems, dirItem)
}

func (client *B2FS) dirItemExists(name string) bool {
	for _, item := range client.dirItems {
		if item.fullPath == name {
			return true
		}
	}
	return false
}

// Serve the B2FS
func Serve(config *Config) error {
	var debug = De.Debug("go-b2-fuse:serve")
	b2, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      config.AccountID,
		ApplicationKey: config.ApplicationKey,
	})
	// b2.Debug = true
	if err != nil {
		fmt.Printf("error while accessing b2 %v", err.Error())
		return fmt.Errorf("B2 API Fail: %v\n", err.Error())
	}
	err = b2.AuthorizeAccount()
	if err != nil {
		fmt.Printf("error while authorizing b2 %v", err.Error())
		return fmt.Errorf("B2 Auth Fail: %v\n", err.Error())
	}
	debug("connected to B2")
	bucket, err := getBucket(b2, config.BucketID)
	if err != nil {
		fmt.Printf("error while getting b2 bucket %v", err.Error())
		return fmt.Errorf("B2 Unable to Get Bucket: %v\n", err.Error())
	}
	debug("got bucket %s", bucket.Name)
	b2fs := &B2FS{
		FileSystem:     pathfs.NewDefaultFileSystem(),
		config:         config,
		b2:             b2,
		bucket:         bucket,
		fetchingFiles:  false,
		fetchingError:  nil,
		filesUpdatedAt: 0,
	}
	nfs := pathfs.NewPathNodeFs(b2fs, nil)
	server, _, err := nodefs.MountRoot(config.MountPoint, nfs.Root(), nil)
	if err != nil {
		fmt.Printf("error during mounting %v", err.Error())
		return fmt.Errorf("Mount fail: %v\n", err.Error())
	}
	debug("serving %v", config.MountPoint)
	server.Serve()
	return nil
}

func getBucket(b2 *backblaze.B2, bucketID string) (*backblaze.Bucket, error) {
	buckets, err := b2.ListBuckets()
	if err != nil {
		return nil, err
	}
	for _, bucket := range buckets {
		if bucket.ID == bucketID {
			return bucket, nil
		}
	}
	return nil, fmt.Errorf("Unable to find bucket %s", bucketID)
}

func getPaths(path string) ([]string, string) {
	dirs := strings.Split(path, string(filepath.Separator))
	list := []string{}
	lastDir := ""
	file := ""
	for i, dir := range dirs {
		lastDir = filepath.Join(lastDir, dir)
		if i == (len(dirs) - 1) {
			return list, lastDir
		}
		list = append(list, lastDir)
	}
	return list, file
}

func escapeName(name string) string {
	name = strings.Replace(name, "._.", "", 1)
	name = strings.Replace(name, "._", "", 1)
	return name
}

func escapePath(path string) string {
	return strings.Trim(path, string(filepath.Separator))
}

func getNowMS() int {
	return time.Now().Nanosecond() / 1000000
}
