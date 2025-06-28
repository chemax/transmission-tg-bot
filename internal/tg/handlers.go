package tg

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"transmission-tg-control/internal/cfg"
	"transmission-tg-control/internal/tr"

	tb "gopkg.in/telebot.v3"
)

const (
	maxMsgLen = 4096 // sendMessage / editMessageText
	maxCBLen  = 200  // answerCallbackQuery
)

type Bot struct {
	tb        *tb.Bot
	conf      *cfg.Config
	tr        *tr.Client
	activeMx  sync.Mutex
	activeMap map[int64]*torrentMeta
	cbMx      sync.Mutex
	cbMap     map[string]cbPayload // token -> payload
	seed      uint64               // атомарный счётчик токенов
	ctx       context.Context
	cancel    context.CancelFunc
}

type cbPayload struct {
	IsMagnet bool
	Payload  string
	Dir      string
	Cat      string // ← НОВОЕ: имя категории
}

type torrentMeta struct {
	chatID int64
	msgID  int
}

func New(conf *cfg.Config, trc *tr.Client) (*Bot, error) {
	tele, err := tb.NewBot(tb.Settings{
		Token:  conf.BotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &Bot{
		tb:        tele,
		conf:      conf,
		tr:        trc,
		activeMap: make(map[int64]*torrentMeta),
		cbMap:     make(map[string]cbPayload), // NEW
		ctx:       ctx,
		cancel:    cancel,
	}
	b.registerHandlers()
	go b.monitorLoop()

	return b, nil
}

func (b *Bot) Start() {
	b.tb.Start()
}

func (b *Bot) Stop() {
	b.cancel()
	b.tb.Stop()
}

// ------------------- handlers -----------------------

func (b *Bot) registerHandlers() {
	b.tb.Handle(tb.OnText, b.onText)
	b.tb.Handle(tb.OnDocument, b.onDocument)
	b.tb.Handle(tb.OnCallback, b.onCallback)
}

func (b *Bot) allowedChat(chatID int64) bool {
	for _, id := range b.conf.ChatWhitelist {
		if chatID == id {
			return true
		}
	}
	return false
}

// --- text (magnet) ---

func (b *Bot) onText(c tb.Context) error {
	if !b.allowedChat(c.Chat().ID) {
		return nil
	}
	text := c.Text()
	if strings.Contains(text, "magnet:?") {
		return b.offerCategories(c, text, true)
	}
	return nil
}

// --- .torrent file ---

func (b *Bot) onDocument(c tb.Context) error {
	if !b.allowedChat(c.Chat().ID) {
		return nil
	}
	doc := c.Message().Document
	ext := strings.ToLower(filepath.Ext(doc.FileName))
	mt := mime.TypeByExtension(ext)
	if ext == ".torrent" || mt == "application/x-bittorrent" {
		return b.offerCategories(c, doc.FileID, false)
	}
	return nil
}

func (b *Bot) trackTorrent(id int64, chatID int64, msgID int) {
	b.activeMx.Lock()
	b.activeMap[id] = &torrentMeta{chatID: chatID, msgID: msgID}
	b.activeMx.Unlock()
}

func (b *Bot) monitorLoop() {
	ticker := time.NewTicker(b.conf.PollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.checkActive()
		case <-b.ctx.Done():
			return
		}
	}
}

func (b *Bot) checkActive() {
	b.activeMx.Lock()
	defer b.activeMx.Unlock()

	for id, meta := range b.activeMap {
		ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
		done, err := b.tr.IsComplete(ctx, id)
		cancel()
		if err != nil || !done {
			continue
		}
		_, _ = b.tb.Reply(&tb.Message{ID: meta.msgID, Chat: &tb.Chat{ID: meta.chatID}},
			fmt.Sprintf("✅ Загрузка %d завершена.", id))
		delete(b.activeMap, id)
	}
}

// internal/tg/bot.go – offerCategories

func (b *Bot) offerCategories(c tb.Context, payload string, isMagnet bool) error {
	var rows [][]tb.InlineButton

	for cat, dir := range b.conf.Categories {
		token := b.nextToken()

		b.cbMx.Lock()
		b.cbMap[token] = cbPayload{
			IsMagnet: isMagnet,
			Payload:  payload,
			Dir:      dir,
			Cat:      cat, // ← сохраняем
		}
		b.cbMx.Unlock()

		btn := tb.InlineButton{
			Text: cat,
			Data: token,
		}
		rows = append(rows, []tb.InlineButton{btn})
	}
	_, err := b.tb.Reply(
		c.Message(),
		"Выберите категорию:",
		&tb.ReplyMarkup{InlineKeyboard: rows},
	)
	return err
}

func (b *Bot) nextToken() string {
	n := atomic.AddUint64(&b.seed, 1)
	return fmt.Sprintf("%x", n) // 8-12 символов
}
func mustLog(err error, msg string) {
	if err != nil {
		log.Printf("[ERR] %s: %v", msg, err)
	}
}

// onCallback обрабатывает нажатия на инлайн-кнопки выбора категории.
func (b *Bot) onCallback(c tb.Context) error {
	// 1. Получаем токен (короткая строка ≤64 байт, без префиксов)
	token := c.Callback().Data

	// 2. Достаём сохранённые данные
	b.cbMx.Lock()
	p, ok := b.cbMap[token]
	if ok {
		delete(b.cbMap, token) // одноразовый токен
	}
	b.cbMx.Unlock()

	if !ok {
		b.safeRespond(c, "Срок действия кнопки истёк")
		return nil
	}

	// 3. Добавляем торрент в Transmission
	ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
	defer cancel()

	var (
		tid int64
		err error
	)

	if p.IsMagnet {
		tid, err = b.tr.AddMagnet(ctx, p.Payload, p.Dir)
	} else {
		// p.Payload содержит FileID .torrent-файла
		fileInfo, errDL := b.tb.FileByID(p.Payload)
		if errDL != nil {
			b.safeRespond(c, "Не смог получить файл")
			return nil
		}
		reader, errDL := b.tb.File(&fileInfo)
		if errDL != nil {
			b.safeRespond(c, "Ошибка загрузки файла")
			return nil
		}
		defer reader.Close()

		buf := new(bytes.Buffer)
		if _, errDL = buf.ReadFrom(reader); errDL != nil {
			b.safeRespond(c, "Ошибка чтения файла")
			return nil
		}
		tid, err = b.tr.AddTorrentFile(ctx, buf.Bytes(), p.Dir)
	}

	if err != nil {
		b.safeRespond(c, "Transmission: "+err.Error())
		return nil
	}

	// 4. Сохраняем в activeMap для мониторинга
	b.trackTorrent(tid, c.Chat().ID, c.Message().ID)

	// 5. Обновляем текст сообщения и убираем клавиатуру
	newText := fmt.Sprintf("📥 Загрузка в категорию *%s* принята.", p.Cat)

	if _, err := b.tb.Edit(
		c.Message(),
		newText,
		&tb.SendOptions{ParseMode: tb.ModeMarkdown},
	); err != nil {
		log.Printf("[ERR] edit msg: %v", err)
	}

	if _, err := b.tb.EditReplyMarkup(c.Message(), nil); err != nil {
		log.Printf("[ERR] clear markup: %v", err)
	}

	// 6. Сообщаем в popup
	b.safeRespond(c, "✅ Добавлено")
	return nil
}

// safeRespond – отправляет popup-ответ на callback, обрезая слишком длинный текст
func (b *Bot) safeRespond(c tb.Context, text string) {
	if len(text) > maxCBLen {
		text = text[:maxCBLen-1] + "…"
	}
	if err := c.Respond(&tb.CallbackResponse{Text: text}); err != nil {
		log.Printf("[ERR] respond: %v", err)
	}
}

// (необязательно) safeSend – короткое chat-сообщение с тем же ограничением
func (b *Bot) safeSend(to tb.Recipient, text string, opts ...interface{}) {
	if len(text) > maxMsgLen {
		text = text[:maxMsgLen-1] + "…"
	}
	if _, err := b.tb.Send(to, text, opts...); err != nil {
		log.Printf("[ERR] send: %v", err)
	}
}
