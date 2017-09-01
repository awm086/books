package main

import (
	"fmt"
	"net/http"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"encoding/xml"
	"github.com/codegangsta/negroni"
	"github.com/yosssi/ace"
	"log"
)

type Page struct {
	Books []Book
}

type Book struct {
	PK 		int
	Title  string
	Author string
	Year   string
	Classification  string
}

type searchResult struct {
	Title  string  `xml:"title,attr"`
	Author string  `xml:"author,attr"`
	Year   string  `xml:"hyr,attr"`
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
	} `xml:"recommendations>ddc>mostPopular"`
}


var db *sql.DB

func verifyDataBase(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if err := db.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w,r)
}
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Print("starting logging")
	mux := http.NewServeMux()
	// db
	db, _ = sql.Open("sqlite3", "goweb.dev")
	defer db.Close()

	fmt.Println("hello world")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template , err := ace.Load("templates/index", "", nil)
		if err != nil  {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		p := Page{Books: []Book{}}

		rows, _ := db.Query("select pk, title, author, classification from books")
		for rows.Next() {
			var b Book
			rows.Scan(&b.PK, &b.Title, &b.Author, &b.Classification)
			p.Books = append(p.Books, b)
		}

		err = template.Execute(w, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/books/add", func(w http.ResponseWriter, r *http.Request) {

		var book ClassifyBookResponse
		var err error


		if book, err = find(r.FormValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// otherwise we have our book we need to save it to the db

		// check if book already added
		result, err := db.Exec("insert into books (pk,title,author,id, classification) values (?,?,?,?,?) ",
			nil, book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		pk, _ := result.LastInsertId()

		b := Book{
			PK:  int(pk),
			Title: book.BookData.Title,
			Author: book.BookData.Author,
			Classification: book.Classification.MostPopular,
		}
		fmt.Println(b)
		if err := json.NewEncoder(w).Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	})

	mux.HandleFunc("/books/delete", func(w http.ResponseWriter, r *http.Request) {

		if _, err := db.Exec("DELETE from books where pk = ?", r.FormValue("pk")); err != nil {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		fmt.Println(r.FormValue("pk"))

	})

		n := negroni.Classic()

	n.Use(negroni.HandlerFunc(verifyDataBase))
	n.UseHandler(mux)

	n.Run(":8080")
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
