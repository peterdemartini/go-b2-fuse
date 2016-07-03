package main

import (
	"fmt"
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/coreos/go-semver/semver"
	"github.com/fatih/color"
	"github.com/peterdemartini/go-b2-fuse/b2fs"
	De "github.com/tj/go-debug"
)

var debug = De.Debug("go-b2-fuse:main")

func main() {
	app := cli.NewApp()
	app.Name = "go-b2-fuse"
	app.Version = version()
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "id, i",
			EnvVar: "GO_B2_FUSE_ACCOUNT_ID",
			Usage:  "B2 Account ID",
		},
		cli.StringFlag{
			Name:   "key, k",
			EnvVar: "GO_B2_FUSE_APPLICATION_KEY",
			Usage:  "B2 Application Key",
		},
		cli.StringFlag{
			Name:   "bucket, b",
			EnvVar: "GO_B2_FUSE_BUCKET_ID",
			Usage:  "B2 Bucket ID",
		},
		cli.StringFlag{
			Name:   "mount, m",
			EnvVar: "GO_B2_FUSE_MOUNT_POINT",
			Usage:  "Fuse Mount Point",
		},
	}
	app.Run(os.Args)
}

func run(context *cli.Context) error {
	b2Config := getOpts(context)
	err := b2fs.Serve(b2Config)
	if err != nil {
		log.Fatalln(err)
		return err
	}
	return nil
}

func getOpts(context *cli.Context) *b2fs.Config {
	accountID := context.String("id")
	applicationKey := context.String("key")
	bucketID := context.String("bucket")
	mountPoint := context.String("mount")

	if accountID == "" {
		cli.ShowAppHelp(context)
		color.Red("  Missing required flag --id, -i or GO_B2_FUSE_ACCOUNT_ID")
		os.Exit(1)
	}

	if applicationKey == "" {
		cli.ShowAppHelp(context)
		color.Red("  Missing required flag --key, -k or GO_B2_FUSE_APPLICATION_KEY")
		os.Exit(1)
	}

	if bucketID == "" {
		cli.ShowAppHelp(context)
		color.Red("  Missing required flag --bucket, -b or GO_B2_FUSE_BUCKET_ID")
		os.Exit(1)
	}

	if mountPoint == "" {
		cli.ShowAppHelp(context)
		color.Red("  Missing required flag --mountPoint, -m or GO_B2_FUSE_MOUNT_POINT")
		os.Exit(1)
	}

	return &b2fs.Config{
		AccountID:      accountID,
		ApplicationKey: applicationKey,
		BucketID:       bucketID,
		MountPoint:     mountPoint,
	}
}

func version() string {
	version, err := semver.NewVersion(VERSION)
	if err != nil {
		errorMessage := fmt.Sprintf("Error with version number: %v", VERSION)
		log.Panicln(errorMessage, err.Error())
	}
	return version.String()
}
