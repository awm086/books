package main

import (
	"fmt"
	"net/http"
	"html/template"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
	"encoding/json"
)

type page struct {
	Name string
	DBstatus bool
}

type searchResult struct {
	Title string
	Author string
	Year string
	ID string
}

func main() {
	template := template.Must(template.ParseFiles("templates/index.html"))
	p := new(page);
	p.Name = "GOPHER";

	// db
	db,_ := sql.Open("sqlite3", "goweb.dev")
	p.DBstatus = db.Ping() == nil

	fmt.Println("hello world")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name");
		if  name != "" {
			p.Name = name
		}

		err := template.ExecuteTemplate(w, "index.html", p)
		if (err != nil) {
			http.Error(w, "something went wrong", http.StatusInternalServerError)
		}
	})


	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		resutls := []searchResult{
			searchResult{"my title", "my author", "1999", "222222"},
			searchResult{"my title", "my author", "1999", "222222"},
		}

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(resutls); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fmt.Println(http.ListenAndServe(":8080", nil))

}
