package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
)

type Weather struct {
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Timezone       string  `json:"timezone"`
	TimezoneOffset int     `json:"timezone_offset"`
	Current        struct {
		Dt         int     `json:"dt"`
		Sunrise    int     `json:"sunrise"`
		Sunset     int     `json:"sunset"`
		Temp       float64 `json:"temp"`
		FeelsLike  float64 `json:"feels_like"`
		Pressure   int     `json:"pressure"`
		Humidity   int     `json:"humidity"`
		DewPoint   float64 `json:"dew_point"`
		Uvi        float64 `json:"uvi"`
		Clouds     int     `json:"clouds"`
		Visibility int     `json:"visibility"`
		WindSpeed  float64 `json:"wind_speed"`
		WindDeg    int     `json:"wind_deg"`
		Weather    []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		} `json:"weather"`
		Rain struct {
			OneH float64 `json:"1h"`
		} `json:"rain"`
	} `json:"current"`
	Minutely []Minutely `json:"minutely"`
	Hourly   []struct {
		Dt         int     `json:"dt"`
		Temp       float64 `json:"temp"`
		FeelsLike  float64 `json:"feels_like"`
		Pressure   int     `json:"pressure"`
		Humidity   int     `json:"humidity"`
		DewPoint   float64 `json:"dew_point"`
		Clouds     int     `json:"clouds"`
		Visibility int     `json:"visibility"`
		WindSpeed  float64 `json:"wind_speed"`
		WindDeg    int     `json:"wind_deg"`
		Weather    []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		} `json:"weather"`
		Pop  float64 `json:"pop"`
		Rain struct {
			OneH float64 `json:"1h"`
		} `json:"rain"`
	} `json:"hourly"`
}

type Minutely struct {
	Dt            int     `json:"dt"`
	Precipitation float64 `json:"precipitation"`
}

type WeatherApi struct {
	ApiKey string
}

func NewWeatherApi(apiKey string) *WeatherApi {
	return &WeatherApi{
		ApiKey: apiKey,
	}
}

func (w *WeatherApi) GetWeather(lat, long float64) (*Weather, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.openweathermap.org/data/2.5/onecall", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("exclude", "daily")
	q.Add("appid", w.ApiKey)
	q.Add("lat", strconv.FormatFloat(lat, 'f', 10, 64))
	q.Add("lon", strconv.FormatFloat(long, 'f', 10, 64))
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var weather Weather
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &weather)

	return &weather, err
}
