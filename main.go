package main

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
)

const (
	RANKED_URL = "https://cdn.wes.cloud/beatstar/bssb/v2-all.json.gz"
	UPDATE_URL = "https://github.com/aplulu/bs-ranked-playlist/releases/latest/download/"
)

type RankedEntry struct {
	Bpm           int               `json:"bpm"`
	Diffs         []RankedEntryDiff `json:"diffs"`
	DownVotes     int               `json:"downVotes"`
	DownloadCount int               `json:"downloadCount"`
	Heat          float64           `json:"heat"`
	Key           string            `json:"key"`
	Mapper        string            `json:"mapper"`
	Rating        float64           `json:"rating"`
	Song          string            `json:"song"`
	UpVotes       int               `json:"upVotes"`
}

type RankedEntryDiff struct {
	Pp     float64 `json:"pp,string"`
	Star   float64 `json:"star,string"`
	Scores int     `json:"scores,string"`
	Diff   string  `json:"diff"`
	Type   int     `json:"type"`
	Len    int     `json:"len"`
	Njs    int     `json:"njs"`
}

type Playlist struct {
	Title       string         `json:"playlistTitle"`
	Author      string         `json:"playlistAuthor"`
	Description string         `json:"playlistDescription"`
	CustomData  map[string]string `json:"customData"`
	Songs       []*PlaylistSong `json:"songs"`
	Image       string         `json:"image,omitempty"`
}

type PlaylistSong struct {
	SongName        string `json:"songName,omitempty"`
	LevelAuthorName string `json:"levelAuthorName,omitempty"`
	Hash            string `json:"hash"`
	Difficulties    []*PlaylistSongDifficulty `json:"difficulties"`
}

type PlaylistSongDifficulty struct {
	Characteristic string `json:"characteristic"`
	Name           string `json:"name"`
}

func main() {
	var (
		imageDir  string
		outputDir string
	)

	flag.StringVar(&imageDir, "image-dir", "images", "Image Directory")
	flag.StringVar(&outputDir, "output-dir", "dist", "Output Directory")
	flag.Parse()

	if _, err := os.Stat(outputDir); err != nil {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create output directory: %v", err)
			os.Exit(1)
			return
		}
	}

	characteristicNames := []string{"Unknown", "Standard", "OneSaber", "NoArrows", "Lightshow", "Degree90", "Degree360", "Lawless"}

	entries, err := downloadRankedList()
	if err != nil {
		panic(err)
	}

	pps := []float64{200, 300, 400, 500}
	songByPP := make(map[float64]map[string]*PlaylistSong)
	for _, pp := range pps {
		songByPP[pp] = make(map[string]*PlaylistSong, 0)
	}

	songByStar := make(map[int]map[string]*PlaylistSong)
	for hash, entry := range entries {
		for _, diff := range entry.Diffs {
			if diff.Star == 0 {
				continue
			}
			star := int(math.Trunc(diff.Star))

			if _, ok := songByStar[star]; !ok {
				songByStar[star] = make(map[string]*PlaylistSong, 0)
			}
			song, ok := songByStar[star][hash]
			if !ok {
				song = &PlaylistSong{
					Hash:         hash,
					Difficulties: make([]*PlaylistSongDifficulty, 0),
				}
				songByStar[star][hash] = song
			}

			if diff.Type > len(characteristicNames) -1 {
				continue
			}

			characteristicName := characteristicNames[diff.Type]
			diffName := diff.Diff
			if diffName == "Expert+" {
				diffName = "ExpertPlus"
			}
			song.Difficulties = append(song.Difficulties, &PlaylistSongDifficulty{
				Characteristic: characteristicName,
				Name:           diffName,
			})
		}

		for _, pp := range pps {
			for _, diff := range entry.Diffs {
				if diff.Pp >= pp {
					song, ok := songByPP[pp][hash]
					if !ok {
						song = &PlaylistSong{
							Hash:         hash,
							Difficulties: make([]*PlaylistSongDifficulty, 0),
						}
						songByPP[pp][hash] = song
					}

					if diff.Type > len(characteristicNames) -1 {
						continue
					}

					characteristicName := characteristicNames[diff.Type]
					diffName := diff.Diff
					if diffName == "Expert+" {
						diffName = "ExpertPlus"
					}
					song.Difficulties = append(song.Difficulties, &PlaylistSongDifficulty{
						Characteristic: characteristicName,
						Name:           diffName,
					})

					break
				}
			}
		}
	}

	// by star
	for star, songMap := range songByStar {
		image, err := getImageByStar(imageDir, star)
		if err != nil {
			panic(err)
		}

		songs := make([]*PlaylistSong, 0)
		for _, s := range songMap {
			songs = append(songs, s)
		}

		of := fmt.Sprintf("ranked_star_%02d.json", star)
		if err := writePlaylist(outputDir, of, fmt.Sprintf("Ranked Songs ★%d", star), "", image, songs); err != nil {
			panic(err)
		}
	}

	// by performance point
	for pp, songMap := range songByPP {
		image, err := getImageByPP(imageDir, int(pp))
		if err != nil {
			panic(err)
		}

		songs := make([]*PlaylistSong, 0)
		for _, s := range songMap {
			songs = append(songs, s)
		}

		of := fmt.Sprintf("ranked_pp_%02d.json", int(pp))
		if err := writePlaylist(outputDir, of, fmt.Sprintf("Ranked Songs %dpp+", int(pp)), "", image, songs); err != nil {
			panic(err)
		}
	}
}

func downloadRankedList() (map[string]RankedEntry, error) {
	log.Printf("Downloading %s...\n", RANKED_URL)
	req, err := http.NewRequest("GET", RANKED_URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got response code %d: %w", resp.StatusCode, err)
	}

	reader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var entries map[string]RankedEntry
	if err := json.NewDecoder(reader).Decode(&entries); err != nil {
		return nil, err
	}

	log.Printf("%d songs found.\n", len(entries))

	return entries, nil
}

func getImageByStar(imageDir string, star int) (string, error) {
	imageFile := fmt.Sprintf("%s/%d.png", imageDir, star)
	if _, err := os.Stat(imageFile); err == nil {
		b, err := ioutil.ReadFile(imageFile)
		if err != nil {
			return "", err
		}

		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
	} else {
		imageFile = imageDir + "/n.png"
		if _, err := os.Stat(imageFile); err == nil {
			b, err := ioutil.ReadFile(imageFile)
			if err != nil {
				return "", err
			}

			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
		}
	}

	return "", nil
}

func getImageByPP(imageDir string, pp int) (string, error) {
	imageFile := fmt.Sprintf("%s/pp_%d.png", imageDir, pp)
	if _, err := os.Stat(imageFile); err == nil {
		b, err := ioutil.ReadFile(imageFile)
		if err != nil {
			return "", err
		}

		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
	} else {
		imageFile = imageDir + "/n.png"
		if _, err := os.Stat(imageFile); err == nil {
			b, err := ioutil.ReadFile(imageFile)
			if err != nil {
				return "", err
			}

			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
		}
	}

	return "", nil
}

func writePlaylist(fileDir string, fileName string, title string, description string, image string, songs []*PlaylistSong) error {
	customData := map[string]string{
		"syncURL": fmt.Sprintf("%s%s", UPDATE_URL, fileName),
	}

	playlist := Playlist{
		Title:       title,
		Author:      "",
		Description: description,
		CustomData:  customData,
		Image:       image,
		Songs:       songs,
	}

	b, err := json.Marshal(playlist)
	if err != nil {
		return err
	}

	outFile := fmt.Sprintf("%s/%s", fileDir, fileName);

	log.Printf( "Writing %s... (%d songs)\n", outFile, len(songs))
	if err := ioutil.WriteFile(outFile, b, 0644); err != nil {
		return err
	}
	return nil
}
