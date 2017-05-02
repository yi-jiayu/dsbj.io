package dsbj

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

var (
	funcMap = template.FuncMap{
		"join": func(a []string) string {
			return strings.Join(a, ", ")
		},
	}
	et = template.Must(template.New("event.html").Funcs(funcMap).ParseFiles("templates/event.html"))
)

type Event struct {
	Id          string
	Title       string
	Description string
	Location    string
	Start       string
	End         string
	POC         string
	Attendees   []string
}

func NewEvent(form url.Values) (error, Event) {
	event := Event{
		Id:          form.Get("id"),
		Title:       form.Get("title"),
		Description: form.Get("description"),
		Location:    form.Get("location"),
		Start:       form.Get("start"),
		End:         form.Get("end"),
		POC:         form.Get("poc"),
	}

	return nil, event
}

func newEvent(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	err := r.ParseForm()
	if err != nil {
		log.Errorf(ctx, "%+v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err, event := NewEvent(r.Form)
	if err != nil {
		log.Errorf(ctx, "%+v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// validate event
	if event.Id == "events" {
		http.Error(w, "Conflict", http.StatusConflict)
	} else if event.Title == "" || event.Description == "" || event.Location == "" ||
		event.Start == "" || event.End == "" || event.POC == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// check if event id is already taken
	if event.Id != "" {
		key := datastore.NewKey(ctx, "Event", event.Id, 0, nil)
		event := new(Event)
		err := datastore.Get(ctx, key, &event)
		if err != nil && err == datastore.ErrNoSuchEntity {

		} else {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}
	}

	key := datastore.NewKey(ctx, "Event", event.Id, 0, nil)
	key, err = datastore.Put(ctx, key, &event)
	if err != nil {
		log.Errorf(ctx, "%+v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var keyString string
	if event.Id == "" {
		keyString = key.Encode()
	} else {
		keyString = event.Id
	}

	http.Redirect(w, r, "/events/"+keyString, http.StatusSeeOther)
}

// GET /events/:id
func getEvent(w http.ResponseWriter, r *http.Request, eventId string) {
	ctx := appengine.NewContext(r)

	// try to decode id as a key, otherwise treat it as a string id
	key, err := datastore.DecodeKey(eventId)
	if err != nil {
		key = datastore.NewKey(ctx, "Event", eventId, 0, nil)
	}

	event := new(Event)
	err = datastore.Get(ctx, key, event)
	if err != nil && err == datastore.ErrNoSuchEntity {
		http.Error(w, "Not Found", http.StatusNotFound)
	}

	if event.Id == "" {
		event.Id = eventId
	}

	err = et.ExecuteTemplate(w, "event.html", event)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func newAttendee(w http.ResponseWriter, r *http.Request, eventId string) {
	ctx := appengine.NewContext(r)

	// get new attendee name from form
	err := r.ParseForm()
	if err != nil {
		log.Errorf(ctx, "%+v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	attendee := r.FormValue("attendee")

	// get event entity from datastore
	// try to decode id as a key, otherwise treat it as a string id
	key, err := datastore.DecodeKey(eventId)
	if err != nil {
		key = datastore.NewKey(ctx, "Event", eventId, 0, nil)
	}

	event := new(Event)
	err = datastore.Get(ctx, key, event)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			log.Errorf(ctx, "%+v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// modify event
	event.Attendees = append(event.Attendees, attendee)

	// add id if the event doesn't have an id
	if event.Id == "" {
		event.Id = eventId
	}

	// update event in datastore
	_, err = datastore.Put(ctx, key, event)
	if err != nil {
		log.Errorf(ctx, "%+v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/events/"+eventId, http.StatusSeeOther)
}

func eventRouter(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	trimmed := strings.Trim(r.URL.Path, "/")
	fragments := strings.Split(trimmed, "/")

	log.Infof(ctx, "%v", fragments)

	switch len(fragments) {
	case 1:
		if r.Method == http.MethodGet {
			if fragments[0] == "events" {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			} else {
				http.Redirect(w, r, "/events/"+fragments[0], http.StatusFound)
				return
			}
		} else if r.Method == http.MethodPost {
			if fragments[0] == "events" {
				// POST /events
				newEvent(w, r)
				return
			} else {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
		}

	case 2:
		if r.Method == http.MethodGet {
			// GET /events/:id
			eventId := fragments[1]
			getEvent(w, r, eventId)
			return
		}
	case 3:
		if r.Method == http.MethodGet {
			// POST /events/:id/attendees

		} else if r.Method == http.MethodPost {
			// GET /events/:id/attendees
			newAttendee(w, r, fragments[1])
			return
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func init() {
	http.HandleFunc("/", eventRouter)
}
