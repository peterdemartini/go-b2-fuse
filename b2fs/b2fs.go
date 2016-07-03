package b2fs

import (
	"log"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// Config defines the configuration
type Config struct {
	AccountID      string
	ApplicationKey string
	BucketID       string
	MountPoint     string
}

// B2FS defines the struct
type B2FS struct {
	pathfs.FileSystem
}

// GetAttr gets file attributes
func (b2fs *B2FS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	switch name {
	case "file.txt":
		return &fuse.Attr{
			Mode: fuse.S_IFREG | 0644, Size: uint64(len(name)),
		}, fuse.OK
	case "":
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}
	return nil, fuse.ENOENT
}

// OpenDir will open directory
func (b2fs *B2FS) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	if name == "" {
		c = []fuse.DirEntry{{Name: "file.txt", Mode: fuse.S_IFREG}}
		return c, fuse.OK
	}
	return nil, fuse.ENOENT
}

// Open will open a file
func (b2fs *B2FS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	if name != "file.txt" {
		return nil, fuse.ENOENT
	}
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}
	return nodefs.NewDataFile([]byte(name)), fuse.OK
}

// Serve the B2FS
func Serve(config *Config) {
	nfs := pathfs.NewPathNodeFs(&B2FS{FileSystem: pathfs.NewDefaultFileSystem()}, nil)
	server, _, err := nodefs.MountRoot(config.MountPoint, nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Serve()
}
