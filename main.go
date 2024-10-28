package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	// The channel that we're sending to
	channel string
	// Global discord instance
	Discord *discordgo.Session
)

type POTASpot struct {
	SpotID       int     `json:"spotId"`
	SpotTime     string  `json:"spotTime"`
	Activator    string  `json:"activator"`
	Frequency    string  `json:"frequency"`
	Mode         *string `json:"mode"` // Use pointer to handle missing fields
	Reference    string  `json:"reference"`
	Spotter      string  `json:"spotter"`
	Source       *string `json:"source"`                 // Use pointer to handle missing fields
	Comments     *string `json:"comments,omitempty"`     // Use pointer to handle missing fields
	Name         *string `json:"name,omitempty"`         // Use pointer to handle missing fields
	LocationDesc *string `json:"locationDesc,omitempty"` // Use pointer to handle missing fields
}

type Spot struct {
	Callsign        string
	Mode            string
	Frequency       string
	Member          bool
	County          string
	CountryCode     string
	POTA            bool
	POTAPark        string
	ParkDescription string
}

type HamalertPayload struct {
	FullCallsign     string `json:"fullCallsign"`
	Callsign         string `json:"callsign"`
	Frequency        string `json:"frequency"`
	Band             string `json:"band"`
	Mode             string `json:"mode"`
	ModeDetail       string `json:"modeDetail"`
	Time             string `json:"time"`
	Spotter          string `json:"spotter"`
	RawText          string `json:"rawText"`
	Title            string `json:"title"`
	Comment          string `json:"comment"`
	Source           string `json:"source"`
	WwffRef          string `json:"wwffRef"`
	WwffDivision     string `json:"wwffDivision"`
	WwffName         string `json:"wwffName"`
	Qsl              string `json:"qsl"`
	Dxcc             int    `json:"dxcc"`
	Entity           string `json:"entity"`
	Cq               string `json:"cq"`
	Continent        string `json:"continent"`
	HomeDxcc         int    `json:"homeDxcc"`
	HomeEntity       string `json:"homeEntity"`
	SpotterDxcc      int    `json:"spotterDxcc"`
	SpotterEntity    string `json:"spotterEntity"`
	SpotterCq        string `json:"spotterCq"`
	SpotterContinent string `json:"spotterContinent"`
	TriggerComment   string `json:"triggerComment"`
}

func getPotaActivations() ([]POTASpot, error) {
	// Get the current spots from pota.app
	resp, err := http.Get("https://api.pota.app/spot/")
	if err != nil {
		log.Println("Failed to get POTA spots\n", err)
		return nil, err
	}
	// Create a map of POTASpots
	var spots []POTASpot
	// Decode the JSON
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&spots); err != nil {
		log.Printf("failed to decode JSON: %w\n", err)
		return nil, err
	}
	// Close the ioreader for the response
	resp.Body.Close()

	return spots, nil
}

func checkSpotRecent(spot Spot) (bool, error) {
	// Check if the spot is recent
	// TODO: Take the spot type and compare to config file
	return false, nil
}
func checkMember(spot Spot) string {
	// Check if the user is a member
	switch strings.ToUpper(spot.Callsign) {
	// TODO: check against env vars
	case "W3LBY":
		return "# <:hamspot:1299208521316962376>__**MEMBER SPOTTED**__<:hamspot:1299208521316962376>"
	default:
		return "## <:hamspot:1299208521316962376> New Spot"
	}
}

func getGuildMembers() []string {
	guildID := os.Getenv("HAM_DISCORD_SPOTTING_BOT_GUILD")
	if guildID == "" {
		return nil
	}

	// Get a list of guild members, paginating
	var paginate_start string
	var member_list []string
	for {
		members, err := Discord.GuildMembers(guildID, paginate_start, 1000)
		if err != nil {
			log.Printf("error retrieving guild members: %v", err)
		}

		for _, member := range members {
			member_list = append(member_list, member.User.Username)
		}

		// If there's <1000 members we don't need to paginate
		if len(members) < 1000 {
			break
		}
		paginate_start = members[len(members)-1].User.ID
	}
	return member_list
}

func sendMessage(channel string, content string) {
	// Send a message to a channel
	message, err := Discord.ChannelMessageSend(channel, content)
	if err != nil {
		log.Println("Something went wrong sending message to discord", err)
	}
	log.Printf("Sent message to %s - message id ", channel, message.ID)
}

func sendSpot(channel string, spot Spot) {
	// Is this a mode we care about
	// TODO: Make this configurable ?
	switch strings.ToLower(spot.Mode) {
	case "ssb", "ft8", "phone", "lsb", "usb":
		log.Printf("Mode %s for Callsign %s on %s is interesting, sending to discord\n", spot.Mode, spot.Callsign, spot.Frequency)
	default:
		log.Printf("Mode (%s) for Callsign (%s) on %s was not interesting, ignoring\n", spot.Mode, spot.Callsign, spot.Frequency)
		return
	}

	// Set header based on if person is a member of discord or not
	header := checkMember(spot)

	// Create a message to send to discord
	message := fmt.Sprintf(`%s
**Callsign:** %s
**Frequency:** %s
**Mode:** %s
`, header, spot.Callsign, spot.Frequency, spot.Mode)
	// Send it to discord
	sendMessage(channel, message)
}

func WebHookHandlerForHamAlert(w http.ResponseWriter, r *http.Request) {
	// Check if the request is a POST
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode JSON
	var payload HamalertPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// Return a HTTP 204 back to the client as we dont send any information back
	w.WriteHeader(http.StatusNoContent)

	// Actually process what we're going to do with the webhook
	// Create a spot instance from payload
	spot := Spot{
		Callsign:  payload.Callsign,
		Mode:      payload.Mode,
		Frequency: payload.Frequency,
	}
	if payload.Source == "POTA" {
		log.Println("Got a POTA spot. Ignoring as we shouldn't get this from ham alert")
	} else {
		go sendSpot(channel, spot)
	}

}

func init() {
	// Load variables from .env only if they are not already set by the OS
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found. Continuing with OS environment variables.")
	}

	// Load bot token
	BotToken := os.Getenv("HAM_DISCORD_SPOTTING_BOT_TOKEN")
	if BotToken == "" {
		log.Fatal("Environment variable HAM_DISCORD_SPOTTING_BOT_TOKEN is not set. Exiting.")
	}

	Discord, err = discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
	// Set the intents for the discord bot
	Discord.Identify.Intents = discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages

}

func UpdateDiscordMemberList() {
	members := getGuildMembers()
	if members != nil {
	}
}

func PotaSpots() {
	spots, err := getPotaActivations()
	if err != nil {
		log.Println("Error getting pota activations,", err)
	}
	for i, pota_spot := range spots {
		if i < 10 {
			// Create spot instance from POTASpot
			spot := Spot{
				Callsign:  pota_spot.Activator,
				Mode:      *pota_spot.Mode,
				Frequency: pota_spot.Frequency,
				POTA:      true,
				POTAPark:  pota_spot.Reference,
			}

			// TODO: Check if the spot is QRT?

			// Check if the spot exists already
			recent, err := checkSpotRecent(spot)
			if err != nil {
				log.Println("Error checking spot is recent", err)
				continue
			}

			if recent {
				// This is a recent spot, lets do nothing
				continue
			}

			sendSpot(channel, spot)
		}
	}
}

func main() {
	// Web hook endpoint for Hamalerts
	ham_alert_url := os.Getenv("HAM_DISCORD_SPOTTING_BOT_HAMALERT_HOOK")
	if ham_alert_url == "" {
		ham_alert_url = "/webhook/hamalert"
	}
	http.HandleFunc(os.Getenv("HAM_DISCORD_SPOTTING_BOT_HAMALERT_HOOK"), WebHookHandlerForHamAlert)

	// Open our Discord Session
	err := Discord.Open()
	if err != nil {
		log.Fatal("error opening connection,", err)
		return
	}
	log.Printf("Connected to discord")

	// Set our channel
	channel = os.Getenv("HAM_DISCORD_SPOTTING_BOT_CHANNEL")
	if channel == "" {
		log.Fatal("Environment variable HAM_DISCORD_SPOTTING_BOT_CHANNEL not set. Unable to send to channels. Exiting.")
	}

	// Start Discord polling for usernames
	go func() {
		// Create a ticker that ticks every 60 minutes
		ticker := time.NewTicker(60 * time.Minute)
		defer ticker.Stop()

		// exec on first run
		UpdateDiscordMemberList()

		for {
			select {
			// Then on each tick run the POTA code
			case <-ticker.C:
				UpdateDiscordMemberList()
			}
		}
	}()

	// Start POTA polling as a separate go func
	go func() {
		// Create a ticker that ticks every 60 seconds
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			// Then on each tick run the POTA code
			case <-ticker.C:
				PotaSpots()
			}
		}
	}()

	// Start a Web server
	addr := os.Getenv("HAM_DISCORD_SPOTTING_BOT_LISTENADDR")
	if addr == "" {
		addr = ":38080"
	}
	log.Fatal(http.ListenAndServe(addr, nil))

	// Wait for a signal before closing
	//sc := make(chan os.Signal, 1)
	//signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	//<-sc

	// Cleanly close down the Discord session.
	Discord.Close()
}
