package telegram

import (
    "fmt"

    tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot — обёртка над tgbot.API для отправки уведомлений
type Bot struct {
    api    *tgbot.BotAPI
    chatID int64
}

// NewBot инициализирует Telegram Bot API и запоминает admin chatID
func NewBot(token string) (*Bot, error) {
    api, err := tgbot.NewBotAPI(token)
    if err != nil {
        return nil, fmt.Errorf("failed to init Telegram bot: %w", err)
    }
    // Для простоты считаем, что chatID = ваш собственный ID (bot.Self.ID)
    return &Bot{api: api, chatID: api.Self.ID}, nil
}

// SendNotification шлёт текстовое уведомление в ваш бот
func (b *Bot) SendNotification(text string) error {
    msg := tgbot.NewMessage(b.chatID, text)
    _, err := b.api.Send(msg)
    return err
}