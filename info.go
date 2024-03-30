package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	DISCORD_TOKEN   string
	SQUAD_SERVER_ID string
	BM_API_URL      *url.URL
)

func init() {
	flag.StringVar(&DISCORD_TOKEN, "t", "", "Discord bot token")
	flag.StringVar(&SQUAD_SERVER_ID, "s", "", "Squad server id")
	flag.Parse()

	var err error
	BM_API_URL, err = url.Parse("https://api.battlemetrics.com/servers/?fields[server]=players,maxPlayers,details")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type BMServer struct {
	Data *BMServerData `json:"data" binding:"required"`
}

type BMServerData struct {
	Id         string              `json:"id" binding:"required"`
	Attributes *BMServerAttributes `json:"attributes" binding:"required"`
}

type BMServerAttributes struct {
	Players    int              `json:"players" binding:"required"`
	MaxPlayers int              `json:"maxPlayers" binding:"required"`
	Details    *BMServerDetails `json:"details" binding:"required"`
}

type BMServerDetails struct {
	Map         string `json:"map" binding:"required"`
	PublicQueue int    `json:"squad_publicQueue" binding:"required"`
}

func main() {
	dg, err := discordgo.New("Bot " + DISCORD_TOKEN)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	go update_status(dg)

	go fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func update_status(discord_session *discordgo.Session) {
	for {
		bmServer, err := get_bm_info(SQUAD_SERVER_ID)
		if err != nil {
			fmt.Println("Ошибка при получении информации из Battlemetrics, ", err)
			time.Sleep(10 * time.Second)
			continue
		}

		var status_text string
		if bmServer.Data.Attributes.Players+bmServer.Data.Attributes.Details.PublicQueue >= bmServer.Data.Attributes.MaxPlayers {
			status_text = fmt.Sprintf(
				"полный сервер, очередь %v, карта %v",
				bmServer.Data.Attributes.Details.PublicQueue,
				bmServer.Data.Attributes.Details.Map,
			)
		} else if bmServer.Data.Attributes.Players < 30 {
			status_text = fmt.Sprintf("SEED, %v игроков", bmServer.Data.Attributes.Players)
		} else {
			status_text = fmt.Sprintf(
				"%v(%v)/%v, карта %v",
				bmServer.Data.Attributes.Players,
				bmServer.Data.Attributes.Details.PublicQueue,
				bmServer.Data.Attributes.MaxPlayers,
				bmServer.Data.Attributes.Details.Map,
			)
		}

		err = discord_session.UpdateGameStatus(0, status_text)
		if err != nil {
			fmt.Println("Ошибка при отправке сообщения в дискорд", err)
		}

		time.Sleep(10 * time.Second)
	}
}

func get_bm_info(server_id string) (*BMServer, error) {
	req, err := http.Get(BM_API_URL.JoinPath(server_id).String())
	if err != nil {
		return nil, err
	}

	defer req.Body.Close()

	if req.StatusCode != 200 {
		return nil, fmt.Errorf("status code is not 200, %v", req.StatusCode)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var bmServer BMServer

	err = json.Unmarshal(body, &bmServer)
	if err != nil {
		return nil, err
	}

	return &bmServer, nil

}
