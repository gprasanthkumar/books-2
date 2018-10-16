package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kjk/notionapi"
)

var (
	useCacheForNotion = true
	// if true, we'll log
	logNotionRequests = true

	notionCacheDir = "notion_cache"
	notionLogDir   = "log"
)

// convert 2131b10c-ebf6-4938-a127-7089ff02dbe4 to 2131b10cebf64938a1277089ff02dbe4
func normalizeID(s string) string {
	return strings.Replace(s, "-", "", -1)
}

func openLogFileForPageID(pageID string) (io.WriteCloser, error) {
	if !logNotionRequests {
		return nil, nil
	}

	name := fmt.Sprintf("%s.go.log.txt", pageID)
	path := filepath.Join(notionLogDir, name)
	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("os.Create('%s') failed with %s\n", path, err)
		return nil, err
	}
	return f, nil
}

func findSubPageIDs(blocks []*notionapi.Block) []string {
	pageIDs := map[string]struct{}{}
	seen := map[string]struct{}{}
	toVisit := blocks
	for len(toVisit) > 0 {
		block := toVisit[0]
		toVisit = toVisit[1:]
		id := normalizeID(block.ID)
		if block.Type == notionapi.BlockPage {
			pageIDs[id] = struct{}{}
			seen[id] = struct{}{}
		}
		for _, b := range block.Content {
			if b == nil {
				continue
			}
			id := normalizeID(block.ID)
			if _, ok := seen[id]; ok {
				continue
			}
			toVisit = append(toVisit, b)
		}
	}
	res := []string{}
	for id := range pageIDs {
		res = append(res, id)
	}
	sort.Strings(res)
	return res
}

func loadPageFromCache(pageID string) *notionapi.Page {
	cachedPath := filepath.Join(notionCacheDir, pageID+".json")
	d, err := ioutil.ReadFile(cachedPath)
	if err != nil {
		return nil
	}

	var page notionapi.Page
	err = json.Unmarshal(d, &page)
	panicIfErr(err)
	return &page
}

// I got "connection reset by peer" error once so retry download 3 times, with a short sleep in-between
func downloadPageRetry(c *notionapi.Client, pageID string) (*notionapi.Page, error) {
	var res *notionapi.Page
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			fmt.Printf("Download %s failed with '%s'\n", pageID, err)
			time.Sleep(3 * time.Second) // not sure if it matters
		}
		res, err = c.DownloadPage(pageID)
		if err == nil {
			return res, nil
		}
	}
	return nil, err
}

func downloadAndCachePage(c *notionapi.Client, pageID string) (*notionapi.Page, error) {
	//fmt.Printf("downloading page with id %s\n", pageID)
	pageID = normalizeID(pageID)
	c.Logger, _ = openLogFileForPageID(pageID)
	if c.Logger != nil {
		defer func() {
			lf := c.Logger.(*os.File)
			lf.Close()
		}()
	}
	cachedPath := filepath.Join(notionCacheDir, pageID+".json")
	page, err := downloadPageRetry(c, pageID)
	if err != nil {
		return nil, err
	}
	d, err := json.MarshalIndent(page, "", "  ")
	if err == nil {
		err = ioutil.WriteFile(cachedPath, d, 0644)
		panicIfErr(err)
	} else {
		// not a fatal error, just a warning
		fmt.Printf("json.Marshal() on pageID '%s' failed with %s\n", pageID, err)
	}
	return page, nil
}

var (
	nNotionPagesFromCache int
)

func loadNotionPage(c *notionapi.Client, pageID string, getFromCache bool, n int) (*notionapi.Page, error) {
	if getFromCache {
		page := loadPageFromCache(pageID)
		if page != nil {
			nNotionPagesFromCache++
			//fmt.Printf("Got %d from cache %s %s\n", n, pageID, page.Root.Title)
			return page, nil
		}
	}
	page, err := downloadAndCachePage(c, pageID)
	if err == nil {
		fmt.Printf("Downloaded %d %s %s\n", n, page.ID, page.Root.Title)
	}
	return page, err
}

func loadNotionPages(c *notionapi.Client, indexPageID string, idToPage map[string]*notionapi.Page, useCache bool) {
	toVisit := []string{indexPageID}

	n := 1
	for len(toVisit) > 0 {
		pageID := normalizeID(toVisit[0])
		toVisit = toVisit[1:]

		if _, ok := idToPage[pageID]; ok {
			continue
		}

		page, err := loadNotionPage(c, pageID, useCache, n)
		panicIfErr(err)
		n++

		idToPage[pageID] = page

		subPages := findSubPageIDs(page.Root.Content)
		toVisit = append(toVisit, subPages...)
	}
}

func loadAllPages(c *notionapi.Client, startIDs []string, useCache bool) map[string]*notionapi.Page {
	idToPage := map[string]*notionapi.Page{}
	nPrev := 0
	for _, startID := range startIDs {
		loadNotionPages(c, startID, idToPage, useCache)
		nDownloaded := len(idToPage) - nPrev
		fmt.Printf("Got %d pages, %d from cache\n", nDownloaded, nNotionPagesFromCache)
		nPrev = len(idToPage)
	}
	return idToPage
}

func rmFile(path string) {
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("os.Remove(%s) failed with %s\n", path, err)
	}
}

func rmCached(pageID string) {
	id := normalizeID(pageID)
	rmFile(filepath.Join(notionLogDir, id+".go.log.txt"))
	rmFile(filepath.Join(notionCacheDir, id+".json"))
}

func createNotionCacheDir() {
	err := os.MkdirAll(notionCacheDir, 0755)
	panicIfErr(err)
}

func createNotionLogDir() {
	if logNotionRequests {
		err := os.MkdirAll(notionLogDir, 0755)
		panicIfErr(err)
	}
}

func createNotionDirs() {
	createNotionLogDir()
	createNotionCacheDir()
}

func removeCachedNotion() {
	err := os.RemoveAll(notionCacheDir)
	panicIfErr(err)
	err = os.RemoveAll(notionLogDir)
	panicIfErr(err)
	createNotionDirs()
}
