package telegram

type sessionStore struct {
	chatRepo  *Repository
	aliasRepo *AliasRepository
}
