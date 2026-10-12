package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/util"
)

func saveTaskImage(task *model.Task, source image.Image) (string, error) {
	directory := taskAttachmentDirectory(task.ID)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("create attachment directory: %w", err)
	}

	baseName := time.Now().Format("20060102-150405.000000000")
	for suffix := 0; suffix < 100; suffix++ {
		fileName := baseName
		if suffix != 0 {
			fileName = fmt.Sprintf("%s-%d", baseName, suffix)
		}
		path := filepath.Join(directory, fileName+".png")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create image attachment: %w", err)
		}

		encodeErr := png.Encode(file, source)
		closeErr := file.Close()
		if encodeErr != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("save image attachment: %w", encodeErr)
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("close image attachment: %w", closeErr)
		}
		return path, nil
	}

	return "", fmt.Errorf("could not choose a unique image attachment name")
}

func taskAttachmentDirectory(taskID int64) string {
	return filepath.Join(util.DataDir(), "attachments", fmt.Sprintf("task-%d", taskID))
}

// removeTaskAttachments only removes the managed directory for a persisted
// task. Arbitrary image paths in imported notes are deliberately untouched.
func removeTaskAttachments(taskID int64) error {
	if taskID <= 0 {
		return nil
	}
	base, err := filepath.Abs(filepath.Join(util.DataDir(), "attachments"))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(taskAttachmentDirectory(taskID))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(base, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to remove attachment path outside the managed directory")
	}
	return os.RemoveAll(target)
}

func imageMarkdown(path string) string {
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, ">", "%3E")
	return "![pasted image](<" + path + ">)"
}
