package bot

import (
	"log/slog"
	"time"

	"github.com/Kishan-Thanki/discord-ping/config"
	"github.com/Kishan-Thanki/discord-ping/database"
	"github.com/bwmarrin/discordgo"
)

var (
	BotID     string
	goBot     *discordgo.Session
	startTime time.Time
)

var slashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "ping",
		Description: "Replies with pong and network latency",
	},
}

func Start() {
	var err error
	startTime = time.Now()

	err = database.InitDB("bot.db")
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		return
	}

	_ = InitFonts()

	goBot, err = discordgo.New("Bot " + config.Token)
	if err != nil {
		slog.Error("Failed to create bot session", "error", err)
		return
	}

	u, err := goBot.User("@me")
	if err != nil {
		slog.Error("Failed to get bot user", "error", err)
		return
	}

	BotID = u.ID

	goBot.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuilds

	goBot.StateEnabled = false

	goBot.AddHandler(messageHandler)
	goBot.AddHandler(slashCommandHandler)
	goBot.AddHandler(welcomeHandler)

	err = goBot.Open()
	if err != nil {
		slog.Error("Failed to open connection", "error", err)
		return
	}

	LoadReminders(goBot)

	_ = goBot.UpdateListeningStatus(config.BotPrefix + "ping")

	for _, cmd := range slashCommands {
		_, err := goBot.ApplicationCommandCreate(goBot.State.User.ID, "", cmd)
		if err != nil {
			slog.Error("Failed to register slash command", "command", cmd.Name, "error", err)
		}
	}

	slog.Info("Bot is running", "user", u.Username, "id", u.ID)
}

func Stop() {
	if goBot != nil {
		slog.Info("Shutting down bot gracefully")
		goBot.Close()
	}
	database.CloseDB()
}
