package main

import (
	"embed"
	"encoding/binary"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB

//go:embed views/*
var viewsfs embed.FS

//go:embed public/*
var publicfs embed.FS

func main() {
	// Initialize the database
	var err error
	db, err = bolt.Open("jairo.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("stats"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	// Views
	var engine *html.Engine
	if isDev() {
		fmt.Println("Running in development mode")
		engine = html.New(".", ".html")
		engine.ShouldReload = true
	} else {
		engine = html.NewFileSystem(http.FS(viewsfs), ".html")
	}
	app := fiber.New(fiber.Config{
		Views:        engine,
		ViewsLayout:  "views/layout",
		ServerHeader: "jairo",
	})
	// Static files
	app.Get("/public/:name", func(c *fiber.Ctx) error {
		ext := path.Ext(c.Params("name"))
		switch ext {
		case ".css":
			c.Set("Content-Type", "text/css")
		case ".js":
			c.Set("Content-Type", "text/javascript")
		case ".png":
			c.Set("Content-Type", "image/png")
		case ".jpg", ".jpeg":
			c.Set("Content-Type", "image/jpeg")
		case ".gif":
			c.Set("Content-Type", "image/gif")
		default:
			fmt.Println("Unknown extension", ext)
		}
		var err error
		var file fs.File
		if isDev() {
			file, err = os.Open("public/" + c.Params("name"))
			if err != nil {
				return c.SendStatus(fiber.StatusNotFound)
			}
			c.Set("Cache-Control", "no-store")
		} else {
			file, err = publicfs.Open("public/" + c.Params("name"))
			if err != nil {
				return c.SendStatus(fiber.StatusNotFound)
			}
			c.Set("Cache-Control", "public, max-age=31536000")
		}
		return c.SendStream(file)
	})
	app.Get("/", Index)
	app.Post("/like", Like)
	app.Listen(":3003")
}

func isDev() bool {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	// when running `go run`, the executable is in a temporary directory.
	return strings.Contains(filepath.Dir(ex), "go-build")
}

func Index(c *fiber.Ctx) error {
	var visits uint64
	var likes uint64
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("stats"))
		likesBytes := b.Get([]byte("likes"))
		if likesBytes != nil {
			likes = binary.LittleEndian.Uint64(likesBytes)
		}
		visitsBytes := b.Get([]byte("visits"))
		if visitsBytes != nil {
			visits = binary.LittleEndian.Uint64(visitsBytes)
		}
		visits++
		return b.Put([]byte("visits"), binary.LittleEndian.AppendUint64(nil, visits))
	})
	if err != nil {
		return err
	}
	return c.Render("views/index", fiber.Map{
		"Visits": visits,
		"Likes":  likes,
	})
}

func Like(c *fiber.Ctx) error {
	err := db.Update(func(tx *bolt.Tx) error {
		var visits uint64
		var likes uint64
		b := tx.Bucket([]byte("stats"))
		visitsBytes := b.Get([]byte("visits"))
		if visitsBytes != nil {
			visits = binary.LittleEndian.Uint64(visitsBytes)
		}
		visits--
		err := b.Put([]byte("visits"), binary.LittleEndian.AppendUint64(nil, visits))
		if err != nil {
			return err
		}
		likesBytes := b.Get([]byte("likes"))
		if likesBytes != nil {
			likes = binary.LittleEndian.Uint64(likesBytes)
		}
		likes++
		return b.Put([]byte("likes"), binary.LittleEndian.AppendUint64(nil, likes))
	})
	if err != nil {
		return err
	}
	return c.RedirectBack("/")
}
