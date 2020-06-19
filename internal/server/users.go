package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	cah "github.com/j4rv/cah/internal/model"
)

const wrongUserOrPassMsg = "The username or password you entered is incorrect."
const notLoggedInMsg = "You need to be logged in to see that page."
const loginRedirect = "/login"
const gameListRedirect = "/games"
const loginFlashKey = "login-flash"

const sessionAge = 60 * 15                    // 15 min
const rememberMeSessionAge = 60 * 60 * 24 * 7 // 1 week

/*
	TEMPLATE HANDLERS
*/

func loginPageHandler(w http.ResponseWriter, req *http.Request) {
	if _, err := userFromSession(w, req); err == nil {
		http.Redirect(w, req, gameListRedirect, http.StatusFound)
		return
	}
	execTemplate(loginPageTmpl, w, http.StatusOK, getFlashes(loginFlashKey, w, req))
}

func processLogin(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	username := req.Form["username"]
	password := req.Form["password"]
	if err := requiredFormParams(username, password); err != nil {
		http.Error(w, missingRequiredParamsMsg, http.StatusUnauthorized)
		return
	}
	u, ok := usecase.User.Login(username[0], password[0])
	if !ok {
		addFlashMsg(wrongUserOrPassMsg, loginFlashKey, w, req)
		http.Redirect(w, req, loginRedirect, http.StatusFound)
		return
	}
	log.Printf("User %s with id %d just logged in!", u.Username, u.ID)
	if err := startSession(u, len(req.Form["rememberme"]) == 1, w, req); err != nil {
		return
	}
	// everything ok, back to index with your brand new session!
	http.Redirect(w, req, gameListRedirect, http.StatusFound)
}

func processRegister(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	username := req.Form["username"]
	password := req.Form["password"]
	if err := requiredFormParams(username, password); err != nil {
		http.Error(w, missingRequiredParamsMsg, http.StatusUnauthorized)
		return
	}
	u, err := usecase.User.Register(username[0], password[0])
	if err != nil {
		addFlashMsg(err.Error(), loginFlashKey, w, req)
		http.Redirect(w, req, loginRedirect, http.StatusFound)
		return
	}
	log.Printf("User %s with id %d just registered!", u.Username, u.ID)
	if err := startSession(u, len(req.Form["rememberme"]) == 1, w, req); err != nil {
		return
	}
	// everything ok, back to index with your brand new session!
	http.Redirect(w, req, gameListRedirect, http.StatusFound)
}

func processLogout(w http.ResponseWriter, req *http.Request) {
	session := getSession(w, req)
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1
	err := session.Save(req, w)
	if err != nil {
		log.Println("Error logging out session", session, ". Err:", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, req, "/", http.StatusFound)
}

func startSession(user cah.User, rememberme bool, w http.ResponseWriter, req *http.Request) error {
	session := getSession(w, req)
	session.Values["user_id"] = user.ID
	if rememberme {
		session.Options.MaxAge = rememberMeSessionAge
	}
	err := session.Save(req, w)
	if err != nil {
		log.Println("Error starting session for user", user.Username, ". Err:", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

/*
	SESSIONS STUFF
*/

var cookies *sessions.CookieStore

func initCookieStore() {
	// In dev mode, use always the same cookie store key
	// This allows us to restart the server and keep the session alive
	if devMode {
		cookies = sessions.NewCookieStore([]byte("dev-mode"))
		return
	}
	// In prod mode, randomize the keys every time the server starts
	skey := securecookie.GenerateRandomKey(64)
	encKey := securecookie.GenerateRandomKey(32)
	cookies = sessions.NewCookieStore(skey, encKey)
	cookies.MaxAge(sessionAge) //15m
}

func userFromSession(w http.ResponseWriter, req *http.Request) (cah.User, error) {
	session := getSession(w, req)
	val, ok := session.Values["user_id"]
	if !ok {
		return cah.User{}, fmt.Errorf("Tried to get user from session without an id")
	}
	id, ok := val.(int)
	if !ok {
		log.Printf("Session with non int id value: '%v'", session.Values)
		return cah.User{}, fmt.Errorf("Session with non int id value")
	}
	u, ok := usecase.User.ByID(id)
	if !ok {
		return u, fmt.Errorf("No user found with ID %d", id)
	}
	session.Save(req, w)
	return u, nil
}

func getSession(w http.ResponseWriter, req *http.Request) *sessions.Session {
	// The CookieStore keys change on every server startup, so we ignore "cookies.Get" errors
	session, _ := cookies.Get(req, "session_token")
	return session
}

func addFlashMsg(msg string, key string, w http.ResponseWriter, req *http.Request) {
	log.Printf("%s got flashed: '%s'", req.RemoteAddr, msg)
	session := getSession(w, req)
	session.AddFlash(msg, key)
	err := session.Save(req, w)
	if err != nil {
		log.Println("ERROR saving session:", err.Error())
	}
}

func getFlashes(key string, w http.ResponseWriter, req *http.Request) []interface{} {
	session := getSession(w, req)
	flashes := session.Flashes(key)
	if len(flashes) == 0 {
		return []interface{}{}
	}
	err := session.Save(req, w)
	if err != nil {
		log.Println("ERROR saving session:", err.Error())
	}
	return flashes
}