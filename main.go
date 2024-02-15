package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB
var engine *html.Engine

//go:embed views/*
var viewsfs embed.FS

//go:embed public/*
var publicfs embed.FS

var visits uint64
var likes uint64

func main() {
	// Initialize the database
	var err error
	db, err = bolt.Open("jairo.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	// Load stats from disk
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("stats"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		b := tx.Bucket([]byte("stats"))
		visitsBytes := b.Get([]byte("visits"))
		if visitsBytes != nil {
			visits = binary.LittleEndian.Uint64(visitsBytes)
		}
		likesBytes := b.Get([]byte("likes"))
		if likesBytes != nil {
			likes = binary.LittleEndian.Uint64(likesBytes)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	// Start the persistor in background
	go persistor()
	// Views
	if isDev() {
		// If we are in dev mode, load views from disk and enable template reload
		fmt.Println("Running in development mode")
		engine = html.New(".", ".html")
		engine.ShouldReload = true
	} else {
		// If we are in production, load views from the embed.FS
		engine = html.NewFileSystem(http.FS(viewsfs), ".html")
	}
	// Create a new Fiber instance
	app := fiber.New(fiber.Config{
		Views:        engine,
		ViewsLayout:  "views/layout",
		ServerHeader: "jairo",
	})
	// Render the index page and cache it
	engine.Render(indexCache, "views/index", fiber.Map{
		"Visits": visits,
		"Likes":  likes,
	}, "views/layout")
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

// isDev returns true if the application is running in development mode
func isDev() bool {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	// when running `go run`, the executable is in a temporary directory.
	return strings.Contains(filepath.Dir(ex), "go-build")
}

// indexCache is a buffer that holds the rendered index page
var indexCache = new(bytes.Buffer)

// Index renders the index page
func Index(c *fiber.Ctx) error {
	// Let's cheat and render the index page in the background
	// while we return the cached version, at the end, speed is all about cheating
	go renderer()
	c.Set("Content-Type", "text/html")
	// Return the cached version
	return c.Send(indexCache.Bytes())
}

func Like(c *fiber.Ctx) error {
	likes++
	return c.RedirectBack("/")
}

// persistor is a background goroutine that persists the stats to disk every 5 seconds
func persistor() {
	for {
		err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("stats"))
			err := b.Put([]byte("visits"), binary.LittleEndian.AppendUint64(nil, visits))
			if err != nil {
				return err
			}
			return b.Put([]byte("likes"), binary.LittleEndian.AppendUint64(nil, likes))
		})
		if err != nil {
			fmt.Println("Error persisting", err)
		}
		log.Println("Stats persisted")
		time.Sleep(5 * time.Second)
	}
}

func renderer() {
	visits++
	// Do a buffer swap to avoid letting the global buffer empty even for a fraction of a second
	indexCacheTemp := new(bytes.Buffer)
	engine.Render(indexCacheTemp, "views/index", fiber.Map{
		"Visits": visits,
		"Likes":  likes,
	}, "views/layout")
	indexCache = indexCacheTemp
}
