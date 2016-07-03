package b2fs

import (
	"fmt"
	"strings"

	"path/filepath"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	De "github.com/tj/go-debug"
	backblaze "gopkg.in/kothar/go-backblaze.v0"
)

var debug = De.Debug("go-b2-fuse:b2fs")

// Config defines the configuration
type Config struct {
	AccountID      string
	ApplicationKey string
	BucketID       string
	MountPoint     string
}

// DirItem is a list item
type DirItem struct {
	item  fuse.DirEntry
	isDir bool
	name  string
	file  backblaze.FileStatus
}

// B2FS defines the struct
type B2FS struct {
	pathfs.FileSystem
	config   *Config
	b2       *backblaze.B2
	bucket   *backblaze.Bucket
	files    []backblaze.FileStatus
	dirItems []DirItem
}

// GetAttr gets file attributes
func (client *B2FS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	debug("get attr %s", name)
	if name == "" {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}
	for _, dirItem := range client.dirItems {
		debug("found file %s == %s", dirItem.name, name)
		if dirItem.name == name {
			debug("matched file %s", dirItem.name)
			return &fuse.Attr{
				Mode: dirItem.item.Mode,
				Size: uint64(dirItem.file.Size),
			}, fuse.OK
		}
	}
	return nil, fuse.ENOENT
}

// OpenDir will open directory
func (client *B2FS) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	debug("open dir %s", name)
	list := []fuse.DirEntry{}
	err := client.getFiles()
	if err != nil {
		fmt.Printf("Error listing file names %v", err)
		return nil, fuse.ENOENT
	}

	for _, dirItem := range client.dirItems {
		dir, _ := filepath.Split(dirItem.name)
		debug("comparing item %s == %s", name, dir)
		if name == dir {
			debug("found item %s == %s", name, dirItem.item.Name)
			list = append(list, dirItem.item)
		}
	}

	debug("got files")
	return list, fuse.OK
}

// Open will open a file
func (client *B2FS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	debug("opening %s", name)
	if name != "file.txt" {
		return nil, fuse.ENOENT
	}
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}
	return nodefs.NewDataFile([]byte(name)), fuse.OK
}

func (client *B2FS) getFiles() error {
	files, err := client.bucket.ListFileNames("", 0)
	if err != nil {
		return err
	}

	client.files = files.Files
	client.dirItems = []DirItem{}

	for _, file := range client.files {
		paths, fullFile := getPaths(file.Name)
		debug("got file %s : %s : %v", file.Name, fullFile, paths)
		client.addDirItem(file, fullFile, false)
		for _, path := range paths {
			client.addDirItem(file, path, true)
		}
	}
	return nil
}

func (client *B2FS) addDirItem(file backblaze.FileStatus, name string, isDir bool) {
	if client.dirItemExists(name) {
		debug("dir item exists %s", name)
		return
	}
	_, fileName := filepath.Split(name)
	var mode uint32
	if isDir {
		mode = fuse.S_IFDIR
	} else {
		mode = fuse.S_IFREG
	}
	item := fuse.DirEntry{
		Name: fileName,
		Mode: mode,
	}
	dirItem := DirItem{
		item:  item,
		name:  name,
		isDir: isDir,
		file:  file,
	}
	client.dirItems = append(client.dirItems, dirItem)
	debug("added dir item %s", name)
}

func (client *B2FS) dirItemExists(name string) bool {
	for _, item := range client.dirItems {
		if item.name == name {
			return true
		}
	}
	return false
}

// Serve the B2FS
func Serve(config *Config) error {
	b2, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      config.AccountID,
		ApplicationKey: config.ApplicationKey,
	})
	if err != nil {
		debug("error during api fail %v", err.Error())
		return fmt.Errorf("B2 API Fail: %v\n", err.Error())
	}
	debug("connected to B2")
	bucket, err := getBucket(b2, config.BucketID)
	if err != nil {
		debug("error during getting bucket %v", err.Error())
		return fmt.Errorf("B2 Unable to Get Bucket: %v\n", err.Error())
	}
	debug("got bucket %s", bucket.Name)
	b2fs := &B2FS{
		FileSystem: pathfs.NewDefaultFileSystem(),
		config:     config,
		b2:         b2,
		bucket:     bucket,
	}
	nfs := pathfs.NewPathNodeFs(b2fs, nil)
	server, _, err := nodefs.MountRoot(config.MountPoint, nfs.Root(), nil)
	if err != nil {
		debug("error during mounting %v", err.Error())
		return fmt.Errorf("Mount fail: %v\n", err.Error())
	}
	debug("serving %v", config.MountPoint)
	server.Serve()
	debug("after serving")
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
