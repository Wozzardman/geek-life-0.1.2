package main

import (
	"image"
	"image/color"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajaxray/geek-life/model"
	"github.com/asdine/storm/v3"
	"github.com/gdamore/tcell/v2"
)

func TestSaveTaskImageAndRenderPreview(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "geek-life-images-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	previousDBFile, hadDBFile := os.LookupEnv("DB_FILE")
	if err := os.Setenv("DB_FILE", filepath.Join(tempDir, "default.db")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if hadDBFile {
			_ = os.Setenv("DB_FILE", previousDBFile)
		} else {
			_ = os.Unsetenv("DB_FILE")
		}
	}()

	source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	source.Set(0, 0, color.NRGBA{R: 255, A: 255})
	source.Set(1, 0, color.NRGBA{G: 255, A: 255})
	source.Set(0, 1, color.NRGBA{B: 255, A: 255})
	source.Set(1, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	path, err := saveTaskImage(&model.Task{ID: 42}, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(tempDir, "attachments", "task-42")) {
		t.Fatalf("attachment path %q is outside the expected task directory", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved attachment is not readable: %v", err)
	}
	if markdown := imageMarkdown(path); !strings.Contains(markdown, filepath.ToSlash(path)) {
		t.Fatalf("Markdown reference %q does not contain the image path", markdown)
	}

	preview := NewImagePreview()
	preview.SetImages([]string{path})
	preview.SetZoom(4)
	if preview.zoom != 4 {
		t.Fatalf("zoom = %d, want 4", preview.zoom)
	}
	preview.Pan(-1, 1)
	if preview.panX != 0 || preview.panY != 1 {
		t.Fatalf("pan was not clamped: %v, %v", preview.panX, preview.panY)
	}
	preview.Fit()
	if preview.zoom != 1 || preview.panX != 0.5 || preview.panY != 0.5 {
		t.Fatalf("fit did not reset the viewport: zoom=%d pan=%v,%v", preview.zoom, preview.panX, preview.panY)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(8, 6)
	preview.SetRect(0, 0, 8, 6)
	preview.Draw(screen)

	blockFound := false
	for y := 1; y < 5; y++ {
		for x := 1; x < 7; x++ {
			mainRune, _, _, _ := screen.GetContent(x, y)
			if mainRune == '▀' {
				blockFound = true
			}
		}
	}
	if !blockFound {
		t.Fatal("image preview did not render any image cells")
	}

	preview.ToggleDetailMode()
	preview.Draw(screen)
	detailCellFound := false
	for y := 1; y < 5; y++ {
		for x := 1; x < 7; x++ {
			mainRune, _, _, _ := screen.GetContent(x, y)
			if mainRune >= '\U0001fb00' && mainRune <= '\U0001fb3b' || mainRune == '▌' || mainRune == '▐' || mainRune == '█' {
				detailCellFound = true
			}
		}
	}
	if !detailCellFound {
		t.Fatal("detail mode did not render any contiguous high-resolution cells")
	}

	database, err := storm.Open(filepath.Join(tempDir, "persistence-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	original := model.Task{Title: "image task", Images: []string{path}}
	if err := database.Save(&original); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	var loaded model.Task
	if err := database.One("ID", original.ID, &loaded); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Images) != 1 || loaded.Images[0] != path {
		t.Fatalf("stored image paths were not preserved: %#v", loaded.Images)
	}
}

func TestRemoveTaskAttachmentsIsScopedToOneTask(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "geek-life-remove-images-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	previousDBFile, hadDBFile := os.LookupEnv("DB_FILE")
	if err := os.Setenv("DB_FILE", filepath.Join(tempDir, "default.db")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if hadDBFile {
			_ = os.Setenv("DB_FILE", previousDBFile)
		} else {
			_ = os.Unsetenv("DB_FILE")
		}
	}()

	target := taskAttachmentDirectory(42)
	neighbor := taskAttachmentDirectory(43)
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(neighbor, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(target, "image.png"), []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeTaskAttachments(42); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target attachment directory still exists: %v", err)
	}
	if _, err := os.Stat(neighbor); err != nil {
		t.Fatalf("neighbor attachment directory was affected: %v", err)
	}
}

func TestSextantRuneMapping(t *testing.T) {
	wants := map[rune]rune{
		0x00: ' ',
		0x01: '\U0001fb00',
		0x15: '▌',
		0x2a: '▐',
		0x3e: '\U0001fb3b',
		0x3f: '█',
	}
	for mask, want := range wants {
		if got := sextantRune(mask); got != want {
			t.Errorf("sextant mask %#x = %U, want %U", mask, got, want)
		}
	}
	seen := make(map[rune]bool)
	for mask := rune(0); mask < 64; mask++ {
		value := sextantRune(mask)
		if seen[value] {
			t.Fatalf("sextant masks are not unique at %#x (%U)", mask, value)
		}
		seen[value] = true
	}
}
