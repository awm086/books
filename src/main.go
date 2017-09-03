package main

import (
	"fmt"
	"net/http"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
	"gopkg.in/gorp.v1"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"encoding/xml"
	"github.com/urfave/negroni"
	"github.com/yosssi/ace"
	"log"
	gmux "github.com/gorilla/mux"
	"github.com/goincremental/negroni-sessions"
	"github.com/goincremental/negroni-sessions/cookiestore"
	"golang.org/x/crypto/bcrypt"
	"strconv"
)

type Page struct {
	Books  []Book
	Filter string
}

type Book struct {
	PK             int64 `db:"pk""`
	Title          string `db:"title"`
	Author         string `db:"author"`
	Classification string `db:"classification"`
	ID             string `db:"id"`
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
var dbmap *gorp.DbMap

func verifyDataBase(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if err := db.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w, r)
}

func initDb() {
	db, _ = sql.Open("sqlite3", "goweb.dev")
	dbmap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}

	dbmap.AddTableWithName(Book{}, "books").SetKeys(true, "pk")
	dbmap.CreateTablesIfNotExists()
}

func getBookCollection(books *[]Book, sort string, filterBy string, writer http.ResponseWriter) bool {
	if sort == "" {
		sort = "pk"
	}
	var where string
	if filterBy == "fiction" {
		where = "where classification between '800' and '900'"
	} else if filterBy == "nonfiction" {
		where = "where classification not between '800' and '900'"
	}
	if _, err := dbmap.Select(books, "select * from books "+where+" order by "+sort); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return false
	}
	return true
}

func getStringFromSession(key string, r *http.Request) string {
	var val string
	if s := sessions.GetSession(r).Get(key); s != nil {
		val = s.(string)
	}

	return val
}
func main() {
	initDb()
	defer dbmap.Db.Close()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Print("starting logging")
	mux := gmux.NewRouter()
	// db

	fmt.Println("hello world")

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {

		if r.FormValue("register") != "" || r.FormValue("login") != "" {
			http.Redirect(w,r,"/",http.StatusFound)
			return
		}

		template, err := ace.Load("templates/login", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = template.Execute(w, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template, err := ace.Load("templates/index", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		p := Page{Books: []Book{}, Filter: getStringFromSession("Filter", r)}
		sortCol := getStringFromSession("sortBy", r)

		if !getBookCollection(&p.Books, sortCol, getStringFromSession("Filter", r), w) {
			return
		}
		if _, err = dbmap.Select(&p.Books, "select * from books"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = template.Execute(w, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		var b []Book
		if !getBookCollection(&b, r.FormValue("sortBy"), getStringFromSession("Filter", r), w) {
			return
		}

		session := sessions.GetSession(r)
		session.Set("sort_by", r.FormValue("sortBy"))

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}).Methods("GET").Queries("sortBy", "{sortBy:title|author|classification}")

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		var b []Book
		if !getBookCollection(&b, r.FormValue("sortBy"), r.FormValue("filter"), w) {
			return
		}

		session := sessions.GetSession(r)
		session.Set("Filter", r.FormValue("filter"))

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}).Methods("GET").Queries("filter", "{filter:all|fiction|nonfiction}")

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
	}).Methods("POST")

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {

		var book ClassifyBookResponse
		var err error

		if book, err = find(r.FormValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// otherwise we have our book we need to save it to the db

		log.Println(book)
		// check if book already added

		b := Book{
			PK:             -1,
			Title:          book.BookData.Title,
			Author:         book.BookData.Author,
			Classification: book.Classification.MostPopular,
		}

		if err = dbmap.Insert(&b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Println(b)
		if err := json.NewEncoder(w).Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	}).Methods("PUT")

	mux.HandleFunc("/books/{pk}", func(w http.ResponseWriter, r *http.Request) {
		pk, err := strconv.ParseInt(gmux.Vars(r)["pk"], 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var b Book
		b.PK = pk

		if _, err = dbmap.Delete(&b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)

	}).Methods("DELETE")

	n := negroni.Classic()

	n.Use(negroni.HandlerFunc(verifyDataBase))
	store := cookiestore.New([]byte("secret123"))
	n.Use(sessions.Sessions("my_session", store))
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
