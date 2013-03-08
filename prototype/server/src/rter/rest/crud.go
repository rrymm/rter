package rest

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"log"
	"net/http"
	"rter/data"
	"rter/storage"
	"strconv"
	"strings"
)

var decoder = schema.NewDecoder()

func RegisterCRUD(r *mux.Router) {
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}", ReadWhere).Methods("GET", "OPTIONS")

	r.HandleFunc("/{datatype:items|users|roles|taxonomy}", Create).Methods("POST", "OPTIONS")

	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}", Read).Methods("GET", "OPTIONS")
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}", Update).Methods("PUT", "OPTIONS")
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}", Delete).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:ranking|direction}", Read).Methods("GET", "OPTIONS")
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:ranking|direction}", Update).Methods("PUT", "OPTIONS")

	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:comments}", ReadWhere).Methods("GET", "OPTIONS")
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:comments}", Create).Methods("POST", "OPTIONS")

	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:comments}/{childkey}", Read).Methods("GET", "OPTIONS")
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:comments}/{childkey}", Update).Methods("PUT", "OPTIONS")
	r.HandleFunc("/{datatype:items|users|roles|taxonomy}/{key}/{childtype:comments}/{childkey}", Delete).Methods("DELETE", "OPTIONS")
}

func Create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

	vars := mux.Vars(r)

	var val interface{}

	types := []string{vars["datatype"]}

	if childtype, ok := vars["childtype"]; ok {
		types = append(types, childtype)
	}

	switch strings.Join(types, "/") {
	case "items":
		val = new(data.Item)
	case "items/comments":
		val = new(data.ItemComment)
	case "users":
		val = new(data.User)
	case "roles":
		val = new(data.Role)
	case "taxonomy":
		val = new(data.Term)
	default:
		http.NotFound(w, r)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&val)

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed json.", http.StatusBadRequest)
		return
	}

	switch v := val.(type) {
	case *data.ItemComment:
		v.ItemID, err = strconv.ParseInt(vars["key"], 10, 64)
	}

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed key in URI", http.StatusBadRequest)
		return
	}

	err = storage.Insert(val)

	if err != nil {
		log.Println(err)
		http.Error(w, "Database error, likely due to malformed request.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(w)
	err = encoder.Encode(val)

	if err != nil {
		log.Println(err)
	}
}

func Read(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

	vars := mux.Vars(r)

	var (
		val interface{}
		err error
	)

	types := []string{vars["datatype"]}

	if childtype, ok := vars["childtype"]; ok {
		types = append(types, childtype)
	}

	switch strings.Join(types, "/") {
	case "items":
		item := new(data.Item)
		item.ID, err = strconv.ParseInt(vars["key"], 10, 64)

		val = item
	case "items/comments":
		comment := new(data.ItemComment)
		comment.ID, err = strconv.ParseInt(vars["childkey"], 10, 64)

		val = comment
	case "users":
		user := new(data.User)
		user.ID, err = strconv.ParseInt(vars["key"], 10, 64)

		val = user
	case "users/direction":
		direction := new(data.UserDirection)
		direction.UserID, err = strconv.ParseInt(vars["key"], 10, 64)

		val = direction
	case "roles":
		role := new(data.Role)
		role.Title = vars["key"]

		val = role
	case "taxonomy":
		term := new(data.Term)
		term.Term = vars["key"]

		val = term
	case "taxonomy/ranking":
		ranking := new(data.TermRanking)
		ranking.Term = vars["key"]

		val = ranking
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed key in URI", http.StatusBadRequest)
		return
	}

	err = storage.Select(val)

	if err == storage.ErrZeroMatches {
		http.Error(w, "No matches for query", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Database error, likely due to malformed request", http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	err = encoder.Encode(val)

	if err != nil {
		log.Println(err)
	}
}

func ReadWhere(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

	vars := mux.Vars(r)

	var (
		val interface{}
		err error
	)

	whereClause := ""
	args := make([]interface{}, 0)

	types := []string{vars["datatype"]}

	if childtype, ok := vars["childtype"]; ok {
		types = append(types, childtype)
	}

	switch strings.Join(types, "/") {
	case "items":
		items := make([]*data.Item, 0)
		val = &items
	case "items/comments":
		comments := make([]*data.ItemComment, 0)

		whereClause = "WHERE ItemID=?"
		var itemID int64
		itemID, err = strconv.ParseInt(vars["key"], 10, 64)
		args = append(args, itemID)

		val = &comments
	case "users":
		users := make([]*data.User, 0)
		val = &users
	case "roles":
		roles := make([]*data.Role, 0)
		val = &roles
	case "taxonomy":
		terms := make([]*data.Term, 0)
		val = &terms
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed key in URI", http.StatusBadRequest)
		return
	}

	err = storage.SelectWhere(val, whereClause, args...)

	if err == storage.ErrZeroMatches {
		http.Error(w, "No matches for query", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Database error, likely due to malformed request", http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	err = encoder.Encode(val)

	if err != nil {
		log.Println(err)
	}
}

func Update(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

	vars := mux.Vars(r)
	var val interface{}

	types := []string{vars["datatype"]}

	if childtype, ok := vars["childtype"]; ok {
		types = append(types, childtype)
	}

	switch strings.Join(types, "/") {
	case "items":
		val = new(data.Item)
	case "items/comments":
		val = new(data.ItemComment)
	case "users":
		val = new(data.User)
	case "users/direction":
		val = new(data.UserDirection)
	case "roles":
		val = new(data.Role)
	case "taxonomy":
		val = new(data.Term)
	case "taxonomy/ranking":
		val = new(data.TermRanking)
	default:
		http.NotFound(w, r)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&val)

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed json.", http.StatusBadRequest)
		return
	}

	switch v := val.(type) {
	case (*data.Item):
		v.ID, err = strconv.ParseInt(vars["key"], 10, 64)
	case (*data.ItemComment):
		v.ID, err = strconv.ParseInt(vars["childkey"], 10, 64)
	case (*data.User):
		v.ID, err = strconv.ParseInt(vars["key"], 10, 64)
	case (*data.UserDirection):
		v.UserID, err = strconv.ParseInt(vars["key"], 10, 64)
	case (*data.Role):
		v.Title = vars["key"]
	case (*data.Term):
		v.Term = vars["key"]
	case (*data.TermRanking):
		v.Term = vars["key"]
	}

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed key in URI", http.StatusBadRequest)
		return
	}

	err = storage.Update(val)

	if err == storage.ErrZeroMatches {
		w.WriteHeader(http.StatusNotModified)
		return
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Database error, likely due to malformed request.", http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	err = encoder.Encode(val)

	if err != nil {
		log.Println(err)
	}
}

func Delete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

	vars := mux.Vars(r)

	var (
		val interface{}
		err error
	)

	types := []string{vars["datatype"]}

	if childtype, ok := vars["childtype"]; ok {
		types = append(types, childtype)
	}

	switch strings.Join(types, "/") {
	case "items":
		item := new(data.Item)
		item.ID, err = strconv.ParseInt(vars["key"], 10, 64)

		val = item
	case "items/comments":
		comment := new(data.ItemComment)
		comment.ID, err = strconv.ParseInt(vars["childkey"], 10, 64)

		val = comment
	case "users":
		user := new(data.User)
		user.ID, err = strconv.ParseInt(vars["key"], 10, 64)

		val = user
	case "roles":
		role := new(data.Role)
		role.Title = vars["key"]

		val = role
	case "taxonomy":
		term := new(data.Term)
		term.Term = vars["key"]

		val = term
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed key in URI", http.StatusBadRequest)
		return
	}

	err = storage.Delete(val)

	if err == storage.ErrZeroMatches {
		http.Error(w, "No matches for query", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Database error, likely due to malformed request", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
