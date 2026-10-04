package app

import (
	"net/http"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/google"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

func googleOAuthHandler(cfg *config.Config, service *google.CalendarService, chats *telegrambot.Repository, bot *telegrambot.Bot, log *logger.Logger, loc *i18n.Localizer) http.HandlerFunc {
	text := func(key string) string {
		if loc != nil {
			return loc.T("ru", key, nil)
		}
		return i18n.New("ru").T("ru", key, nil)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, text("oauth_missing_code"), http.StatusBadRequest)
			return
		}

		serviceName, peerID, err := telegrambot.DecodeGoogleState(r.URL.Query().Get("state"))
		if err != nil {
			http.Error(w, text("oauth_missing_state"), http.StatusBadRequest)
			return
		}

		credentials, email, err := service.Exchange(r.Context(), code)
		if err != nil {
			log.Error().Err(err).Msg("google oauth exchange failed")
			http.Error(w, text("oauth_exchange_failed"), http.StatusBadRequest)
			return
		}

		if err := chats.SaveGoogleAccount(&telegrambot.GoogleAccount{
			Email:              email,
			RefreshToken:       credentials.RefreshToken,
			AccessToken:        credentials.AccessToken,
			AccessTokenExpires: credentials.Expiry,
		}); err != nil {
			log.Error().Err(err).Msg("failed to save google account")
			http.Error(w, text("oauth_save_failed"), http.StatusInternalServerError)
			return
		}

		chat, err := chats.FindOrCreate(serviceName, peerID)
		if err != nil {
			log.Error().Err(err).Msg("failed to load chat for google link")
		} else {
			chat.GoogleEmail = email
			if err := chats.Save(chat); err != nil {
				log.Error().Err(err).Msg("failed to link google account to chat")
			}
		}

		if err := bot.SendGoogleLinked(peerID, email); err != nil {
			log.Error().Err(err).Msg("failed to notify chat about google link")
		}

		w.Write([]byte(text("oauth_linked_ok")))
	}
}
