package commands

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/nomikai"
)

// In-memory storage for active scheduled task timers
type activeTask struct {
	ID        int
	Command   string
	Time      time.Time
	Repeat    bool
	ChannelID string
	GuildID   int64
	UserID    string
}

var (
	activeTasks = make(map[int]*activeTask)
	tasksMu     sync.Mutex
)

// RestoreScheduledTasks loads all scheduled tasks from the database and schedules them
func RestoreScheduledTasks(ctx context.Context, s *discordgo.Session, svc *nomikai.Service, database *db.DB) error {
	dbTasks, err := database.ListAllScheduledTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to load scheduled tasks: %w", err)
	}

	now := time.Now()
	tasksMu.Lock()
	defer tasksMu.Unlock()

	for _, dbTask := range dbTasks {
		// Skip tasks that are already past (non-repeating)
		if dbTask.Time.Before(now) && !dbTask.Repeat {
			// Delete expired non-repeating tasks
			if err := database.DeleteScheduledTask(ctx, dbTask.ID); err != nil {
				log.Printf("Failed to delete expired task %d: %v", dbTask.ID, err)
			}
			continue
		}

		// For repeating tasks that are past, update to next occurrence
		if dbTask.Time.Before(now) && dbTask.Repeat {
			// Calculate next occurrence efficiently
			daysPast := int(now.Sub(dbTask.Time).Hours() / 24)
			daysToAdd := daysPast + 1
			nextTime := dbTask.Time.Add(time.Duration(daysToAdd) * 24 * time.Hour)
			dbTask.Time = nextTime
			if err := database.UpdateScheduledTaskTime(ctx, dbTask.ID, nextTime); err != nil {
				log.Printf("Failed to update task %d time: %v", dbTask.ID, err)
				continue
			}
		}

		// Create active task
		task := &activeTask{
			ID:        dbTask.ID,
			Command:   dbTask.Command,
			Time:      dbTask.Time,
			Repeat:    dbTask.Repeat,
			ChannelID: dbTask.ChannelID,
			GuildID:   dbTask.GuildID,
			UserID:    dbTask.UserID,
		}
		activeTasks[task.ID] = task
		go scheduleTask(s, svc, database, task)
	}

	log.Printf("Restored %d scheduled tasks", len(activeTasks))
	return nil
}

func HandleJikan(s *discordgo.Session, i *discordgo.InteractionCreate, svc *nomikai.Service, database *db.DB) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondText(s, i, "サブコマンドを指定してください")
		return
	}

	subCmd := options[0]

	switch subCmd.Name {
	case "add":
		handleJikanAdd(s, i, subCmd.Options, svc, database)
	case "list":
		handleJikanList(s, i)
	}
}

func handleJikanAdd(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption, svc *nomikai.Service, database *db.DB) {
	cmdStr := getStringOption(options, "command")
	timeStr := getStringOption(options, "time")
	repeatOpt := getBoolOption(options, "repeat")

	if cmdStr == nil || timeStr == nil {
		respondText(s, i, "コマンドと時間を指定してください")
		return
	}

	isRepeat := false
	if repeatOpt != nil {
		isRepeat = *repeatOpt
	}

	targetTime, err := parseTime(*timeStr)
	if err != nil {
		respondText(s, i, fmt.Sprintf("時間の形式が正しくありません: %v (例: 18:00, 2025-12-26 18:00)", err))
		return
	}

	now := time.Now()
	if targetTime.Before(now) {
		respondText(s, i, "指定された時間は既に過ぎています")
		return
	}

	channelID := i.ChannelID
	guildID := i.GuildID
	userID := i.Member.User.ID

	// Parse guildID to int64
	gid, err := strconv.ParseInt(guildID, 10, 64)
	if err != nil {
		respondText(s, i, "ギルドIDの解析に失敗しました")
		return
	}

	// Save to database
	ctx := context.Background()
	dbTask, err := database.AddScheduledTask(ctx, *cmdStr, targetTime, isRepeat, channelID, gid, userID)
	if err != nil {
		respondText(s, i, fmt.Sprintf("タスクの保存に失敗しました: %v", err))
		return
	}

	// Register active task in memory
	tasksMu.Lock()
	task := &activeTask{
		ID:        dbTask.ID,
		Command:   dbTask.Command,
		Time:      dbTask.Time,
		Repeat:    dbTask.Repeat,
		ChannelID: dbTask.ChannelID,
		GuildID:   dbTask.GuildID,
		UserID:    dbTask.UserID,
	}
	activeTasks[dbTask.ID] = task
	tasksMu.Unlock()

	scheduleTask(s, svc, database, task)

	msg := fmt.Sprintf("ID: %d\nコマンド `%s` を %s に実行するように予約しました", dbTask.ID, *cmdStr, targetTime.Format("2006-01-02 15:04"))
	if isRepeat {
		msg += "（毎日繰り返し）"
	}
	respondText(s, i, msg)
}

func handleJikanList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Parse guildID to int64
	gid, err := strconv.ParseInt(i.GuildID, 10, 64)
	if err != nil {
		respondText(s, i, "ギルドIDの解析に失敗しました")
		return
	}

	tasksMu.Lock()
	defer tasksMu.Unlock()

	if len(activeTasks) == 0 {
		respondText(s, i, "予約されているコマンドはありません")
		return
	}

	var b strings.Builder
	b.WriteString("予約コマンド一覧:\n")
	
	for _, t := range activeTasks {
		if t.GuildID != gid {
			continue
		}

		repeatStr := ""
		if t.Repeat {
			repeatStr = " (毎日)"
		}
		fmt.Fprintf(&b, "- ID: %d | %s | `%s`%s\n", t.ID, t.Time.Format("2006-01-02 15:04"), t.Command, repeatStr)
	}

	if b.Len() == len("予約コマンド一覧:\n") {
		respondText(s, i, "このサーバーで予約されているコマンドはありません")
		return
	}

	respondText(s, i, b.String())
}

func scheduleTask(s *discordgo.Session, svc *nomikai.Service, database *db.DB, task *activeTask) {
	now := time.Now()
	duration := task.Time.Sub(now)

	// Validate that duration is positive, otherwise schedule immediately
	if duration < 0 {
		duration = 0
	}

	time.AfterFunc(duration, func() {
		// Execute
		guildIDStr := strconv.FormatInt(task.GuildID, 10)
		executeScheduledCommand(s, svc, database, task.ChannelID, guildIDStr, task.UserID, task.Command)

		tasksMu.Lock()
		defer tasksMu.Unlock()

		// Check if task still exists in memory
		if _, exists := activeTasks[task.ID]; !exists {
			return
		}

		// Use timeout context for database operations
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if task.Repeat {
			// Update time for next run
			task.Time = task.Time.Add(24 * time.Hour)
			// Update database
			if err := database.UpdateScheduledTaskTime(ctx, task.ID, task.Time); err != nil {
				log.Printf("Failed to update scheduled task time: %v", err)
				return
			}
			go scheduleTask(s, svc, database, task)
		} else {
			// Remove from database
			if err := database.DeleteScheduledTask(ctx, task.ID); err != nil {
				log.Printf("Failed to delete scheduled task: %v", err)
			}
			// Remove from memory
			delete(activeTasks, task.ID)
		}
	})
}

func parseTime(input string) (time.Time, error) {
	now := time.Now()
	
	// Try HH:MM format
	if t, err := time.ParseInLocation("15:04", input, time.Local); err == nil {
		target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if target.Before(now) {
			target = target.Add(24 * time.Hour)
		}
		return target, nil
	}

	// Try YYYY-MM-DD HH:MM format
	if t, err := time.ParseInLocation("2006-01-02 15:04", input, time.Local); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unsupported format")
}

func executeScheduledCommand(s *discordgo.Session, svc *nomikai.Service, database *db.DB, channelID, guildIDStr, userID, cmdStr string) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return
	}

	mainCmd := parts[0]

	// Check for custom command (starts with !)
	if strings.HasPrefix(mainCmd, "!") && len(mainCmd) > 1 {
		cmdName := mainCmd[1:]
		gid, _ := strconv.ParseInt(guildIDStr, 10, 64)
		cmd, err := database.GetCommand(context.Background(), gid, cmdName)
		if err == nil && cmd != nil {
			s.ChannelMessageSend(channelID, cmd.Response)
			return
		}
	}
	
	switch mainCmd {
	case "nomikai":
		if len(parts) < 2 {
			s.ChannelMessageSend(channelID, "nomikai コマンドにはサブコマンドが必要です")
			return
		}
		subCmd := parts[1]
		ctx := context.Background()

		switch subCmd {
		case "start":
			gid, _ := strconv.ParseInt(guildIDStr, 10, 64)
			err := svc.StartSession(ctx, channelID, gid, userID, 1, "organizer")
			if err != nil {
				s.ChannelMessageSend(channelID, fmt.Sprintf("予約実行エラー (nomikai start): %v", err))
			} else {
				s.ChannelMessageSend(channelID, "予約実行: 飲み会セッションを開始しました")
			}
		case "stop":
			err := svc.StopSession(ctx, channelID)
			if err != nil {
				s.ChannelMessageSend(channelID, fmt.Sprintf("予約実行エラー (nomikai stop): %v", err))
			} else {
				s.ChannelMessageSend(channelID, "予約実行: 飲み会セッションを終了しました")
			}
		default:
			s.ChannelMessageSend(channelID, fmt.Sprintf("予約実行: 未対応の nomikai サブコマンドです: %s", subCmd))
		}
	default:
		// For other commands, just send the message
		s.ChannelMessageSend(channelID, cmdStr)
	}
}
