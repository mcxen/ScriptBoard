package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"scriptboard/src/internal/cache"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A script's pictures (a wallpaper feed, inspiration boards) are kept: each
// picture is downloaded once and cached per script, and the script's history of
// pictures, newest first, is kept with the results. The carousel shows the
// history. Resetting the script's data clears both.

const (
	// imageHistoryMax is how many pictures a script's history keeps.
	imageHistoryMax = 200
	// imageKey is the saved history of pictures, under the script's results.
	imageKey = "images-history"
	// imageTimeout bounds one download.
	imageTimeout = 20 * time.Second
)

// imageHistory is a script's saved history of pictures, newest first.
func (a *app) imageHistory(id string) []engine.Image {
	if a.cache == nil {
		return nil
	}
	data, _, ok := a.cache.GetResultOf(id, imageKey)
	if !ok {
		return nil
	}
	var list []engine.Image
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}

// fetchImage keeps a copy of the picture at url for script id, and returns its
// path, or "" when it cannot be had.
func (a *app) fetchImage(id, url string) string {
	if path, ok := a.cache.BlobPath(id, url); ok {
		return path
	}
	ctx, cancel := context.WithTimeout(context.Background(), imageTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, cache.MaxBlobBytes+1))
	if err != nil || len(data) == 0 || len(data) > cache.MaxBlobBytes {
		return ""
	}
	path, err := a.cache.PutBlob(id, url, data)
	if err != nil {
		return ""
	}
	return path
}

// keepImages merges a run's pictures into the script's history: each new one
// gets its copy, a picture already in the history moves to the front, and the
// history is cut to imageHistoryMax, with the copies of the pictures cut off
// removed. It returns the history, for the carousel.
func (a *app) keepImages(sc store.Script, fresh []engine.Image, now time.Time) []engine.Image {
	if a.cache == nil {
		return fresh
	}
	old := a.imageHistory(sc.ID)
	if len(fresh) == 0 && len(old) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var list []engine.Image
	add := func(img engine.Image) {
		if img.URL == "" || seen[img.URL] {
			return
		}
		seen[img.URL] = true
		list = append(list, img)
	}
	for _, img := range fresh {
		img.Local = a.fetchImage(sc.ID, img.URL)
		add(img)
	}
	for _, img := range old {
		if path, ok := a.cache.BlobPath(sc.ID, img.URL); ok {
			img.Local = path
		} else {
			img.Local = ""
		}
		add(img)
	}
	for _, dropped := range list[min(len(list), imageHistoryMax):] {
		_ = a.cache.ForgetBlob(sc.ID, dropped.URL)
	}
	if len(list) > imageHistoryMax {
		list = list[:imageHistoryMax]
	}
	if data, err := json.Marshal(list); err == nil {
		_ = a.cache.PutResultOf(sc.ID, imageKey, data, now)
	}
	return list
}

// clearImages forgets a script's pictures: the copies and the history.
func (a *app) clearImages(sc store.Script) {
	if a.cache == nil {
		return
	}
	for _, img := range a.imageHistory(sc.ID) {
		_ = a.cache.ForgetBlob(sc.ID, img.URL)
	}
	if data, err := json.Marshal([]engine.Image{}); err == nil {
		_ = a.cache.PutResultOf(sc.ID, imageKey, data, a.now())
	}
	a.invalidate(sc.ID)
	a.status = tr("已清空图片历史")
}
