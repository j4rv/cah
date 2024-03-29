package usecase

import (
	"github.com/j4rv/cah"
	"github.com/j4rv/cah/db/mem"
)

func getGameUsecase() cah.GameUsecases {
	store := mem.GetGameStore()
	return NewGameUsecase(store)
}
