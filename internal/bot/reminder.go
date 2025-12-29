package bot

import (
	"context"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/nomikai"
)

// reminderWorker periodically posts unpaid settlement reminders to channels.
type reminderWorker struct {
	db       *db.DB
	nomikai  *nomikai.Service
	session  reminderSession
	stopChan chan struct{}
	ticker   *time.Ticker
	interval time.Duration
}

// Minimal session interface for sending channel messages.
type reminderSession interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

func newReminderWorker(session reminderSession, database *db.DB, svc *nomikai.Service) *reminderWorker {
	return &reminderWorker{
		db:       database,
		nomikai:  svc,
		session:  session,
		stopChan: make(chan struct{}),
		interval: time.Minute,
	}
}

func (w *reminderWorker) start() {
	if w == nil {
		return
	}
	w.ticker = time.NewTicker(w.interval)
	go w.loop()
}

func (w *reminderWorker) stop() {
	if w == nil {
		return
	}
	close(w.stopChan)
	if w.ticker != nil {
		w.ticker.Stop()
	}
}

func (w *reminderWorker) loop() {
	ctx := context.Background()
	for {
		select {
		case <-w.ticker.C:
			w.tick(ctx)
		case <-w.stopChan:
			return
		}
	}
}

func (w *reminderWorker) tick(ctx context.Context) {
	now := time.Now()
	targets, err := w.db.DueReminders(ctx, now)
	if err != nil {
		log.Printf("reminder: failed to load due reminders: %v", err)
		return
	}

	for _, t := range targets {
		msg, err := w.nomikai.ReminderMessageByEventID(ctx, t.EventID)
		if err != nil {
			log.Printf("reminder: failed to build message for event %d: %v", t.EventID, err)
			continue
		}
		if msg == "" {
			continue
		}
		autoMsg := msg + "\n\n※このメッセージは自動投稿です"
		if err := w.sendWithRetry(ctx, t.ChannelID, autoMsg); err != nil {
			log.Printf("reminder: failed to send message to channel %s: %v", t.ChannelID, err)
			// Back off so we don't hammer Discord (or a bad edge) every minute.
			backoff := 2 * time.Minute
			if t.IntervalMinutes > 0 {
				max := time.Duration(t.IntervalMinutes) * time.Minute
				if backoff > max {
					backoff = max
				}
			}
			next := now.Add(backoff)
			if derr := w.db.DelayReminder(ctx, t.EventID, next); derr != nil {
				log.Printf("reminder: failed to delay reminder for event %d: %v", t.EventID, derr)
			}
			continue
		}
		next := now.Add(time.Duration(t.IntervalMinutes) * time.Minute)
		if err := w.db.MarkReminderSent(ctx, t.EventID, now, next); err != nil {
			log.Printf("reminder: failed to mark reminder sent for event %d: %v", t.EventID, err)
		}
	}
}

func (w *reminderWorker) sendWithRetry(ctx context.Context, channelID, content string) error {
	const attemptTimeout = 12 * time.Second
	const maxAttempts = 2

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		sendCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		_, err := w.session.ChannelMessageSend(channelID, content, discordgo.WithContext(sendCtx))
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTemporaryOrTimeout(err) {
			return err
		}
		time.Sleep(time.Duration(300+rand.Intn(500)) * time.Millisecond)
	}
	return lastErr
}

func isTemporaryOrTimeout(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok {
		return ne.Timeout() || ne.Temporary()
	}
	return false
}
