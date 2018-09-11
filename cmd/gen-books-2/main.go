package main

import (
	"flag"
	"fmt"

	"github.com/kjk/notionapi"
)

var (
	// "https://www.notion.so/kjkpublic/Essential-Go-2cab1ed2b7a44584b56b0d3ca9b80185"
	notionGoStartPage = "2cab1ed2b7a44584b56b0d3ca9b80185"

	flgNoCache bool
)

var (
	books = []string{
		"Go", "Essential Go", notionGoStartPage,
	}
)

func parseCmdLineFlags() {
	flag.BoolVar(&flgNoCache, "no-cache", false, "if true, disables cache for notion")
	flag.Parse()
}

func downloadBook(bookShortName, bookName, notionStartPageID string) *Book {
	idToPage := map[string]*notionapi.Page{}
	loadNotionPages(notionGoStartPage, idToPage, !flgNoCache)
	fmt.Printf("Loaded %d pages for book %s\n", len(idToPage), bookName)
	book := bookFromPages(notionStartPageID, idToPage)
	book.Title = bookShortName
	book.TitleLong = bookName
	return book
}

func iterPages(book *Book, onPage func(*Page) bool) {
	processed := map[string]bool{}
	toProcess := []*Page{book.RootPage}
	for len(toProcess) > 0 {
		page := toProcess[0]
		toProcess = toProcess[1:]
		id := normalizeID(page.NotionPage.ID)
		if processed[id] {
			continue
		}
		processed[id] = true
		toProcess = append(toProcess, page.Pages...)
		shouldContinue := onPage(page)
		if !shouldContinue {
			return
		}
	}
}

func buildIDToPage(book *Book) {
	book.idToPage = map[string]*Page{}
	fn := func(page *Page) bool {
		id := normalizeID(page.NotionPage.ID)
		book.idToPage[id] = page
		return true
	}
	iterPages(book, fn)
}

func bookPagesToHTML(book *Book) {
	nProcessed := 0
	fn := func(page *Page) bool {
		notionToHTML(page.NotionPage, book)
		nProcessed++
		return true
	}
	iterPages(book, fn)
	fmt.Printf("bookPagesToHTML: processed %d pages for book %s\n", nProcessed, book.TitleLong)
}

func genBookFiles(book *Book) {
	buildIDToPage(book)
	bookPagesToHTML(book)
}

func main() {
	parseCmdLineFlags()

	//flgNoCache = true

	nBooks := len(books) / 3
	panicIf(nBooks*3 != len(books), "bad definition of books")
	//maybeRemoveNotionCache()
	for i := 0; i < nBooks; i++ {
		book := downloadBook(books[i*3], books[i*3+1], books[i*3+2])
		genBookFiles(book)
	}
}