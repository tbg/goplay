package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const getVINsRequest = `
{"locale":"de-DE","isLegacyTos":false,"limit":999,"location":{"latitude":50.7419457,"longitude":7.0950367},"radius":200,"vehicleService":{"id":0},"vehicleType":{"id":44}}
`

const getVinsEndpoint = `https://www.tuv.com/tos-pti-relaunch-2019/rest/ajax/getMergedLocationsByGeo`

func main() {
	if err := mainE(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mainE() error {
	var canMakeAppointment struct {
		Iresult int
	}
	if err := query(`https://www.tuv.com/tos-pti-relaunch-2019/rest/ajax/getIsDriversLicenseBookingValid`,
		`{
  "locale": "de-DE",
  "sFirstname": "Tobias",
  "sLastname": "Grieger",
  "sBirthdate": "08.09.1987",
  "isLegacyTos": false
}`, &canMakeAppointment,
	); err != nil {
		return err
	}
	if canMakeAppointment.Iresult == 1 {
		fmt.Println("can't make appointment yet")
	}
	d := time.Now().Add(14 * 24 * time.Hour)
	vics, err := vics()
	if err != nil {
		return err
	}
	fmt.Printf("Querying %d locations\n", len(vics))
	for i, hits := 0, 0; hits <= 5 && i <= 30; i++ {
		date := d.Add(time.Duration(i) * 24 * time.Hour).Format("2006-01-02")
		apps, err := appointments(date, vics)
		if err != nil {
			return err
		}
		if len(apps) > 0 {
			fmt.Printf("Appointments on %s:\n", date)
			for _, app := range apps {
				fmt.Printf("%s %dkm away in %s\n", app.Time, app.KM, app.Where)
			}
			hits++
			continue
		}
		fmt.Printf("Nothing on %s\n", date)
	}
	return nil
}

type Vic struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Zip            string  `json:"zip"`
	Distance       float64 `json:"distance"`
	ExternalLocale string  `json:"externalLocale"`
}

type getVinsResp struct {
	Vics []Vic
}

const vacanciesByDayEndpoint = `https://www.tuv.com/tos-pti-relaunch-2019/rest/ajax/getVacanciesByDay`

type ID struct {
	ID int `json:"id"`
}

type Date struct {
	Date string `json:"date"`
}

type vacanciesByDayReq struct {
	Locale          string `json:"locale"`          // de-DE
	VehicleServices []ID   `json:"vehicleServices"` // [4007]
	VehicleType     ID     `json:"vehicleType"`     // 44
	Vics            []Vic  `json:"vics"`
	Date            Date   `json:"date"` // YYYY-MM-DD
}

type vacanciesByDayResp struct {
	Timetables []struct {
		Vic                Vic
		TimeRangeVacancies []struct {
			Timeslots []struct {
				AvailableDates []string
			}
		}
	}
}

func query(endpoint string, req, resp interface{}) error {
	var reader io.Reader
	if s, ok := req.(string); ok {
		reader = strings.NewReader(s)
	} else {
		b, err := json.Marshal(req)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	r, err := http.NewRequest("POST", endpoint, reader)
	r.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	rr, err := client.Do(r)
	if err != nil {
		return err
	}
	defer rr.Body.Close()
	b, _ := io.ReadAll(rr.Body)

	if rr.StatusCode != http.StatusOK {
		return errors.New(string(b))
	}

	return json.Unmarshal(b, resp)
}

type appointment struct {
	Date  string
	Time  string
	Where string
	KM    int
}

func vics() ([]Vic, error) {
	var vr getVinsResp
	if err := query(getVinsEndpoint, getVINsRequest, &vr); err != nil {
		return nil, err
	}
	return vr.Vics, nil
}

func appointments(date string, vics []Vic) ([]appointment, error) {

	vicsByID := map[int]Vic{}
	for _, v := range vics {
		vicsByID[v.ID] = v
	}

	dr := vacanciesByDayReq{
		Locale:          "de-DE",
		VehicleServices: []ID{{4007}},
		VehicleType:     ID{44},
		Vics:            vics,
		Date:            Date{date},
	}

	var drr vacanciesByDayResp
	if err := query(vacanciesByDayEndpoint, dr, &drr); err != nil {
		return nil, err
	}

	var sl []appointment
	for _, r := range drr.Timetables {
		for _, v := range r.TimeRangeVacancies {
			for _, s := range v.Timeslots {
				for _, d := range s.AvailableDates {
					sl = append(sl, appointment{
						Date:  date,
						Time:  d,
						Where: vicsByID[r.Vic.ID].Name,
						KM:    int(vicsByID[r.Vic.ID].Distance),
					})
				}
			}
		}
	}

	return sl, nil
}
