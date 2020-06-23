package usecase

import (
	"errors"
	"fmt"
	"log"

	"github.com/j4rv/cah/pkg/rng"

	cah "github.com/j4rv/cah/internal/model"
)

var nilBlackCard = &cah.BlackCard{}

type errorEmptyBlackDeck struct{}

func (e errorEmptyBlackDeck) Error() string {
	return "Zero cards left in black deck"
}

type stateController struct {
	store   cah.GameStateStore
	usecase *cah.Usecases
}

// NewGameStateUsecase returns a cah.GameStateUsecases
func NewGameStateUsecase(uc *cah.Usecases, store cah.GameStateStore) cah.GameStateUsecases {
	return &stateController{store: store, usecase: uc}
}

func (control stateController) Create() *cah.GameState {
	ret := &cah.GameState{
		Players:         []*cah.Player{},
		HandSize:        10,
		DiscardPile:     []*cah.WhiteCard{},
		WhiteDeck:       []*cah.WhiteCard{},
		BlackDeck:       []*cah.BlackCard{},
		BlackCardInPlay: nilBlackCard,
	}
	ret, err := control.store.Create(ret)
	if err != nil {
		log.Println("Error while creating a new game:", err)
	}
	return ret
}

func (control stateController) ByID(id int) (*cah.GameState, error) {
	return control.store.ByID(id)
}

func (control stateController) End(gs *cah.GameState) error {
	if gs.Phase == cah.Finished {
		return errors.New("Tried to end a game but it has already finished")
	}
	gs.Phase = cah.Finished
	err := control.store.Update(gs)
	if err != nil {
		return err
	}
	g, err := control.usecase.Game.ByGameStateID(gs.ID)
	if err != nil {
		return err
	}
	g.Finished = true
	err = control.usecase.Game.Update(g)
	if err != nil {
		return err
	}
	return nil
}

// TODO this method needs some heavy refactoring
func (control stateController) GiveBlackCardToWinner(wID uint, g *cah.GameState) error {
	if err := giveBlackCardToWinnerChecks(wID, g); err != nil {
		return err
	}
	var winner *cah.Player
	for _, p := range g.Players {
		if p.User.ID == wID {
			winner = p
		}
	}
	if winner == nil {
		return fmt.Errorf("Invalid winner id %d", wID)
	}
	winner.Points = append(winner.Points, g.BlackCardInPlay)
	// the rest of the code should be "roundStart" or "startNewRound"
	if g.MaxRounds > 0 && g.CurrRound >= g.MaxRounds {
		return control.End(g)
	}
	if (len(g.BlackDeck)) == 0 || (len(g.WhiteDeck)) == 0 {
		return control.End(g)
	}
	g.BlackCardInPlay = nilBlackCard
	for _, p := range g.Players {
		p.WhiteCardsInPlay = []*cah.WhiteCard{}
	}
	_ = control.nextCzar(g)
	err := putBlackCardInPlay(g)
	if err != nil {
		return err
	}
	playersDraw(g)
	err = control.store.Update(g)
	if err != nil {
		return err
	}
	return nil
}

func giveBlackCardToWinnerChecks(w uint, s *cah.GameState) error {
	if s.Phase != cah.CzarChoosingWinner {
		return fmt.Errorf("Tried to choose a winner in a non valid phase '%s'", s.Phase)
	}
	for i, p := range s.Players {
		if i == s.CurrCzarIndex {
			continue
		}
		if len(p.WhiteCardsInPlay) != s.BlackCardInPlay.Blanks {
			return errors.New("Not all sinners have played their cards")
		}
	}
	return nil
}

// PlayWhiteCards checks that the player is able to play those cards in the current gamestate, then calls playWhiteCards
func (control stateController) PlayWhiteCards(p int, cs []int, g *cah.GameState) error {
	if checkErr := playWhiteCardsChecks(p, g); checkErr != nil {
		return checkErr
	}
	if len(cs) != g.BlackCardInPlay.Blanks {
		return fmt.Errorf("Invalid amount of white cards to play, expected %d but got %d",
			g.BlackCardInPlay.Blanks,
			len(cs))
	}
	return control.playWhiteCards(p, cs, g)
}

func (control stateController) playWhiteCards(i int, cs []int, gs *cah.GameState) error {
	player := gs.Players[i]
	newCardsPlayed, err := player.ExtractCardsFromHand(cs)
	if err != nil {
		return err
	}
	player.WhiteCardsInPlay = append(player.WhiteCardsInPlay, newCardsPlayed...)
	if control.AllSinnersPlayedTheirCards(gs) {
		gs.Phase = cah.CzarChoosingWinner
	}
	err = control.store.Update(gs)
	if err != nil {
		return err
	}
	return nil
}

func (control stateController) PlayRandomWhiteCards(i int, g *cah.GameState) error {
	if checkErr := playWhiteCardsChecks(i, g); checkErr != nil {
		return checkErr
	}
	cardIndexes := rng.RandomDifferentInts(g.BlackCardInPlay.Blanks, 0, len(g.Players[i].Hand))
	log.Printf("Player %d played random cards: %v", i, cardIndexes)
	return control.playWhiteCards(i, cardIndexes, g)
}

func (stateController) AllSinnersPlayedTheirCards(s *cah.GameState) bool {
	for i, p := range s.Players {
		if i == s.CurrCzarIndex {
			continue
		}
		if len(p.WhiteCardsInPlay) != s.BlackCardInPlay.Blanks {
			return false
		}
	}
	return true
}

func playersDraw(s *cah.GameState) {
	for _, p := range s.Players {
		for len(p.Hand) < s.HandSize {
			p.Hand = append(p.Hand, s.DrawWhite())
		}
	}
}

func putBlackCardInPlay(g *cah.GameState) error {
	if err := putBlackCardInPlayChecks(g); err != nil {
		return err
	}
	g.BlackCardInPlay = g.BlackDeck[0]
	g.BlackDeck = g.BlackDeck[1:]
	g.Phase = cah.SinnersPlaying
	g.CurrRound++
	return nil
}

func putBlackCardInPlayChecks(g *cah.GameState) error {
	if g.BlackCardInPlay != nilBlackCard {
		return errors.New("Tried to put a black card in play but there is already a black card in play")
	}
	if g.Phase == cah.Finished {
		return errors.New("Tried to put a black card in play but the game has already finished")
	}
	if len(g.BlackDeck) == 0 {
		return errorEmptyBlackDeck{}
	}
	return nil
}

func (stateController) nextCzar(gs *cah.GameState) error {
	if gs.BlackCardInPlay != nilBlackCard {
		return errors.New("Tried to rotate to the next Czar but there is still a black card in play")
	}
	if gs.Phase == cah.Finished {
		return errors.New("Tried to rotate to the next Czar but the game has already finished")
	}
	gs.CurrCzarIndex++
	if int(gs.CurrCzarIndex) == len(gs.Players) {
		gs.CurrCzarIndex = 0
	}
	return nil
}

func playWhiteCardsChecks(i int, g *cah.GameState) error {
	if i < 0 || int(i) >= len(g.Players) {
		return errors.New("Non valid sinner index")
	}
	if i == g.CurrCzarIndex {
		return errors.New("The Czar cannot play white cards")
	}
	if len(g.Players[i].WhiteCardsInPlay) != 0 {
		return errors.New("You played your card(s) already")
	}
	return nil
}
