package main

import (
	"log"
	"math/rand"
	"os/exec"
	"path"
	"time"

	"github.com/j4rv/cah"
	"github.com/j4rv/cah/db/mem"
	"github.com/j4rv/cah/db/sqlite"
	"github.com/j4rv/cah/server"
	"github.com/j4rv/cah/usecase"
	"github.com/j4rv/cah/usecase/fixture"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	run()
}

func run() {
	buildFrontend()
	sqlite.InitDB("database.sqlite3")
	stateStore := mem.GetGameStateStore()
	gameStore := mem.GetGameStore()
	cardStore := mem.GetCardStore()
	userStore := sqlite.NewUserStore()
	usecases := cah.Usecases{
		GameState: usecase.NewGameStateUsecase(stateStore),
		Card:      usecase.NewCardUsecase(cardStore),
		User:      usecase.NewUserUsecase(userStore),
		Game:      usecase.NewGameUsecase(gameStore),
	}
	populateCards(usecases.Card)

	fixture.PopulateUsers(usecases.User)
	createTestGames(usecases)

	server.Start(usecases)
}

func buildFrontend() {
	command := exec.Command("npm", "run", "build")
	command.Dir = path.Join(cah.FrontendDir)
	npmOut, err := command.Output()
	log.Println(string(npmOut))
	if err != nil {
		log.Panic(err)
	}
}

// For quick prototyping

func createTestGames(usecase cah.Usecases) {
	users := getTestUsers(usecase)
	usecase.Game.Create(users[1], "A long and descriptive game name", "")
	usecase.Game.Create(users[0], "Amo a juga", "1234")
	// Start the Amo a juga game
	g, _ := usecase.Game.ByID(2)
	usecase.Game.UserJoins(users[1], g)
	g, _ = usecase.Game.ByID(2)
	usecase.Game.UserJoins(users[2], g)
	g, _ = usecase.Game.ByID(2)
	wd := usecase.Card.ExpansionWhites("Base-UK")
	bd := usecase.Card.ExpansionBlacks("Base-UK")
	state := usecase.GameState.Create()
	err := usecase.Game.Start(g, state,
		usecase.Game.Options().BlackDeck(bd),
		usecase.Game.Options().WhiteDeck(wd),
	)
	if err != nil {
		panic(err)
	}
}

func getTestUsers(usecase cah.Usecases) []cah.User {
	users := make([]cah.User, 4)
	for i := 0; i < 4; i++ {
		u, _ := usecase.User.ByID(i + 1)
		users[i] = u
	}
	return users
}

func populateCards(cardUC cah.CardUsecases) {
	cardUC.CreateFromFolder(cah.AppDir+"/expansions/base-uk", "Base-UK")
	cardUC.CreateFromFolder(cah.AppDir+"/expansions/anime", "Anime")
	cardUC.CreateFromFolder(cah.AppDir+"/expansions/kikis", "Kikis")
	cardUC.CreateFromFolder(cah.AppDir+"/expansions/expansion-1", "The First Expansion")
	cardUC.CreateFromFolder(cah.AppDir+"/expansions/expansion-2", "The Second Expansion")
	// to check that it does not break the app
	cardUC.CreateFromFolder(cah.AppDir+"/expansinos/undefined", "Non existant")
}