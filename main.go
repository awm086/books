package main

import (
	"fmt"
	"net/http"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/lib/pq"
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
	"os"
)

type Page struct {
	Books  []Book
	Filter string
	User   string
}

type LoginPage struct {
	Error string
}

type Book struct {
	PK             int64 `db:"pk""`
	Title          string `db:"title"`
	Author         string `db:"author"`
	Classification string `db:"classification"`
	ID             string `db:"id"`
	User           string `db:"user"`
}

type User struct {
	Username string `db:"username"`
	Secrete  []byte `db:"secret"`
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
	if os.Getenv("ENV") != "production" {
		db, _ = sql.Open("sqlite3", "goweb.dev")
		dbmap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
	} else {
		db, _ = sql.Open("postgres", os.Getenv("DATABASE_URL"))
		dbmap = &gorp.DbMap{Db: db, Dialect: gorp.PostgresDialect{}}
	}
	dbmap.AddTableWithName(Book{}, "books").SetKeys(true, "pk")
	dbmap.AddTableWithName(User{}, "users").SetKeys(false, "username")
	dbmap.CreateTablesIfNotExists()

}

func getBookCollection(books *[]Book, sort, filterBy, username string, writer http.ResponseWriter) bool {
	if sort == "" {
		sort = "pk"
	}
	var where string
	where = " Where \"user\"= " + dbmap.Dialect.BindVar(0)
	if filterBy == "fiction" {
		where = " AND classification between '800' and '900'"
	} else if filterBy == "nonfiction" {
		where = " AND classification not between '800' and '900'"
	}
	if _, err := dbmap.Select(books, "select * from books "+where+" order by "+sort, username); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return false
	}
	return true
}

func verifyUser(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.URL.Path == "/login" {
		next(w, r)
		return
	}

	// verify user exist in db
	if username := getStringFromSession("User", r); username != "" {
		log.Print("user name:" + username)
		if user, _ := dbmap.Get(User{}, username); user != nil {
			next(w, r)
			return
		}
	}

	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

func getStringFromSession(key string, r *http.Request) string {
	log.Print("kye:" + key)
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
		var p LoginPage
		if r.FormValue("register") != "" {
			secret, _ := bcrypt.GenerateFromPassword([]byte(r.FormValue("password")), bcrypt.DefaultCost)
			user := User{Username: r.FormValue("username"), Secrete: secret}
			if err := dbmap.Insert(&user); err != nil {
				p.Error = err.Error()
			} else {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}

		if r.FormValue("login") != "" {
			log.Print("user login")
			user, err := dbmap.Get(User{}, r.FormValue("username"))
			if err != nil {
				p.Error = err.Error()
			} else if user == nil {
				p.Error = "User does not exist" + r.FormValue("username")
			} else {
				u := user.(*User)
				if err = bcrypt.CompareHashAndPassword(u.Secrete, []byte(r.FormValue("password"))); err != nil {
					p.Error = err.Error()
				} else {

					sessions.GetSession(r).Set("User", u.Username)
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
			}

		}

		template, err := ace.Load("templates/login", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = template.Execute(w, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		sessions.GetSession(r).Set("User", nil)
		sessions.GetSession(r).Set("Filter", nil)
		http.Redirect(w, r, "/login", http.StatusFound)

	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template, err := ace.Load("templates/index", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		p := Page{Books: []Book{}, Filter: getStringFromSession("Filter", r), User: getStringFromSession("User", r)}
		sortCol := getStringFromSession("sortBy", r)

		if !getBookCollection(&p.Books, sortCol, getStringFromSession("Filter", r), getStringFromSession("User", r), w) {
			return
		}
		/*if _, err = dbmap.Select(&p.Books, "select * from books"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}*/

		err = template.Execute(w, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		var b []Book

		if !getBookCollection(&b, r.FormValue("sortBy"), getStringFromSession("Filter", r), getStringFromSession("User", r), w) {
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
		if !getBookCollection(&b, r.FormValue("sortBy"), r.FormValue("filter"), getStringFromSession("User", r), w) {
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
			ID:             r.FormValue("id"),
			User:           getStringFromSession("User", r),
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
		//query := "select * from books  where pk = " + dbmap.Dialect.BindVar(0) + " AND  \"user\"= " + dbmap.Dialect.BindVar(1)
		// if err := dbmap.SelectOne(&b, query, getStringFromSession("User", r)); err != nil {
		// 	http.Error(w, err.Error(), http.StatusBadRequest)
		// 	log.Println("del book not for user")
		// 	return
		// }

		if _, err = dbmap.Delete(&b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)

	}).Methods("DELETE")

	n := negroni.Classic()

	store := cookiestore.New([]byte("secret123"))
	n.Use(sessions.Sessions("my_session", store))
	n.Use(negroni.HandlerFunc(verifyDataBase))
	n.Use(negroni.HandlerFunc(verifyUser))
	n.UseHandler(mux)

	port := os.Getenv("port")
	if port == "" {
		port = "8080"
	}
	n.Run(":" + port)
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
