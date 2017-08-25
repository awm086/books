package main

import (
	"fmt"
	"net/http"
	"html/template"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"encoding/xml"
)

type page struct {
	Name     string
	DBstatus bool
}

type searchResult struct {
	Title  string  `xml:"title,attr"`
	Author string  `xml:"author,attr"`
	Year   string  `xml:"hyr>attr"`
	ID     string  `xml:"owi,attr"`
}

type ClassifySearchResp struct {
	Results []searchResult `xml:"works>work"`
}

type ClassifyBookResponse struct {
	BookData struct {
		Title  string  `xml:"title,attr"`
		Author string  `xml:"author,attr"`
		ID     string  `xml:"owi>attr"`
	} `xml:"work""`
	Classification struct {
		MostPopular string `xml:"nsfa,attr"`
	} `xml:"recommendations>dcc>mostpopular"`
}

func main() {
	template := template.Must(template.ParseFiles("templates/index.html"))
	p := new(page)
	p.Name = "GOPHER"

	// db
	db, _ := sql.Open("sqlite3", "goweb.dev")

	fmt.Println("hello world")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name");
		if name != "" {
			p.Name = name
		}

		err := template.ExecuteTemplate(w, "index.html", p)
		if (err != nil) {
			http.Error(w, "something went wrong", http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var results []searchResult
		var err error

		if results, err = search(r.FormValue("search")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/books/add", func(w http.ResponseWriter, r *http.Request) {

		var book ClassifyBookResponse
		var err error

		if err = db.Ping(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		if book, err = find(r.FormValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// otherwise we have our book we need to save it to the db

		fmt.Println(book)
		// check if book already added
		var count int
		c, err := db.Query("select COUNT (*) from books where id = ?", book.BookData.ID)
		for c.Next() {
			c.Scan(&count)
		}
		fmt.Println(count)
		_, err = db.Exec("insert into books (pk,title,author,id, classification) values (?,?,?,?,?) ",
			nil, book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	})

	fmt.Println(http.ListenAndServe(":8080", nil))

}

func search(query string) ([]searchResult, error) {
	var c ClassifySearchResp
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?&summary=true&title=" + url.QueryEscape(query))

	if err != nil {
		return []searchResult{}, err
	}
	err = xml.Unmarshal(body, &c)
	return c.Results, err
}

func find(id string) (ClassifyBookResponse, error) {
	var c ClassifyBookResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?&summary=true&owi=" + url.QueryEscape(id))
	if err != nil {
		return ClassifyBookResponse{}, err
	}
	err = xml.Unmarshal(body, &c)
	return c, err

}
func classifyAPI(url string) ([]byte, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get(url); err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)

}
