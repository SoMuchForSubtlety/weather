package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/MemeLabs/dggchat"
	"github.com/davecgh/go-spew/spew"
	"github.com/somuchforsubtlety/weather/geo"
	"github.com/somuchforsubtlety/weather/geo/locationiq"
)

type bot struct {
	failTimeout  time.Duration
	sgg          *dggchat.Session
	msgBuffer    chan msg
	api          *WeatherApi
	geo          geo.Geocoder
	lastMsgTime  time.Time
	lastMsg      string
	nick         string
	userRequests map[string]time.Time
}

type msg struct {
	message   string
	private   bool
	recipient dggchat.User
}

type config struct {
	AuthToken     string `json:"auth_token,omitempty"`
	Address       string `json:"address,omitempty"`
	WeatherApiKey string `json:"weather_api_key,omitempty"`
	GeoApiKey     string `json:"geo_api_key,omitempty"`
	Nick          string `json:"nick,omitempty"`
}

var configFile string

func main() {
	flag.Parse()
	logFile, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logFile.Close()

	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	config, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}

	bot := newBot(config)

	log.Println("[INFO] trying to establish connection...")
	err = bot.sgg.Open()
	if err != nil {
		log.Fatal("[FATAL]", err)
	}
	log.Println("[INFO] connected")

	bot.sgg.AddPMHandler(bot.onPM)
	bot.sgg.AddMessageHandler(bot.onMessage)
	bot.sgg.AddErrorHandler(onError)

	for {
		msg := <-bot.msgBuffer
		if msg.private {
			err := bot.sgg.SendPrivateMessage(msg.recipient.Nick, msg.message)
			if err != nil {
				log.Printf("[ERROR] could not send private messate: %s", err)
			}
		} else {
			if msg.message == bot.lastMsg {
				msg.message += "."
			}
			bot.lastMsg = msg.message
			err := bot.sgg.SendMessage(msg.message)
			if err != nil {
				log.Printf("[ERROR] could not send message: %s", err)
			}

		}
		time.Sleep(time.Millisecond * 450)
	}
}

func (b *bot) onPM(dm dggchat.PrivateMessage, s *dggchat.Session) {
	m := dggchat.Message{Sender: dm.User, Timestamp: dm.Timestamp, Message: dm.Message}
	b.answer(m, true)
}

func (b *bot) onMessage(m dggchat.Message, s *dggchat.Session) {
	if strings.HasPrefix(strings.TrimSpace(m.Message), b.nick) {
		b.answer(m, false)
	}
}

//noinspection GoUnusedParameter
func onError(e string, session *dggchat.Session) {
	log.Printf("[ERROR] %s", e)
}

func readConfig() (*config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bv, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var c *config

	err = json.Unmarshal(bv, &c)
	if err != nil {
		return nil, err
	}

	return c, err
}

func newBot(config *config) *bot {
	var b bot
	var err error
	b.nick = config.Nick
	b.failTimeout = time.Second * 30
	b.sgg, err = dggchat.New(";jwt=" + config.AuthToken)
	if err != nil {
		log.Fatalf("Unable to get connect to chat: %v", err)
	}
	b.userRequests = make(map[string]time.Time)
	u, err := url.Parse(config.Address)
	if err != nil {
		log.Fatalf("[ERROR] can't parse url %v", err)
	}
	b.sgg.SetURL(*u)
	b.msgBuffer = make(chan msg, 100)
	b.api = NewWeatherApi(config.WeatherApiKey)
	// 3 	country
	// 5 	state
	// 8 	county
	// 10 	city
	// 14 	suburb
	// 16 	street
	// 18 	building
	b.geo = locationiq.Geocoder(config.GeoApiKey, 18)
	return &b
}

func (b *bot) answer(message dggchat.Message, private bool) {
	if !private {
		sub := time.Since(b.lastMsgTime)
		if sub < time.Second*10 {
			log.Printf("throttled, last request %s ago", sub.String())
			return
		}
		last, ok := b.userRequests[message.Sender.Nick]
		sub = time.Since(last)
		if ok && sub < time.Minute && strings.ToLower(message.Sender.Nick) != "somuchforsubtlety" {
			log.Printf("user %s throttled, last request %s ago", message.Sender.Nick, sub.String())
			return
		}

		b.lastMsgTime = time.Now()
	}
	b.userRequests[message.Sender.Nick] = time.Now()
	prvt := "public"
	if private {
		prvt = "private"
	}
	searchText := strings.TrimSpace(strings.Replace(message.Message, b.nick, "", -1))
	log.Printf("[INFO] received %s request from [%s]: %q", prvt, message.Sender.Nick, searchText)

	location, err := b.geo.Geocode(searchText)
	if err != nil {
		log.Println(err)
		return
	}
	weather, err := b.api.GetWeather(location.Lat, location.Lng)
	if err != nil {
		log.Println(err)
		return
	}

	if len(weather.Minutely) > 0 {
		b.sendMsg(buildChart(condense(weather.Minutely), location.Name), private, message.Sender)
	} else if len(weather.Hourly) > 0 {
		spew.Dump(weather.Hourly)
		b.sendMsg(fmt.Sprintf("%.2f mms of rain expected over the next hour in %s", weather.Hourly[0].Rain.OneH, location.Name), private, message.Sender)
		log.Printf("unable to get weather for %s", searchText)
	}
}

func buildChart(rains []Minutely, locationName string) string {
	var max float64
	var min float64
	var total float64
	var forecast string
	for _, rain := range rains {
		if rain.Precipitation > max {
			max = rain.Precipitation
		}
		if rain.Precipitation < min {
			min = rain.Precipitation
		}
		total += rain.Precipitation
	}

	if max == 0 {
		return "no rain expexted over the next hour in " + locationName
	}

	forecast += "["
	for _, rain := range rains {
		forecast += string(lookupBlock(rain.Precipitation, max))
	}
	forecast += "]"
	forecast += fmt.Sprintf(" %.3f mm of rain over the next hour in %s", total, locationName)

	return forecast
}

func condense(rains []Minutely) []Minutely {
	var condensed []Minutely
	acc := 0.00
	for i, rain := range rains {
		acc += rain.Precipitation
		if i > 0 && i%5 == 0 {
			condensed = append(condensed, Minutely{rain.Dt, acc / 5.0})
			acc = 0
		}
	}
	return condensed
}

func lookupBlock(val, max float64) rune {
	percent := val / max

	if percent == 0 {
		return ' '
	} else if percent < 0.125 {
		return '▁'
	} else if percent < 0.25 {
		return '▂'
	} else if percent < 0.375 {
		return '▃'
	} else if percent < 0.5 {
		return '▄'
	} else if percent < 0.625 {
		return '▅'
	} else if percent < 0.75 {
		return '▆'
	} else if percent < 0.875 {
		return '▇'
	} else {
		return '█'
	}
}

func init() {
	flag.StringVar(&configFile, "config", "./config/config.json", "location of config")
}

func (b *bot) sendMsg(message string, private bool, user dggchat.User) {
	var m msg
	m.private = private
	m.recipient = user
	m.message = message

	b.msgBuffer <- m
}
