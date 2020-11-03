package main

import (
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
	RANKED_URL = "https://cdn.wes.cloud/beatstar/bssb/v2-ranked.json"
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
	Songs       []PlaylistSong `json:"songs"`
	Image       string         `json:"image,omitempty"`
}

type PlaylistSong struct {
	SongName        string `json:"songName,omitempty"`
	LevelAuthorName string `json:"levelAuthorName,omitempty"`
	Hash            string `json:"hash"`
}

func main() {
	var (
		imageDir  string
		outputDir string
	)

	flag.StringVar(&imageDir, "image-dir", "images", "Image Directory")
	flag.StringVar(&outputDir, "output-dir", "dist", "Output Directory")
	flag.Parse()

	entries, err := downloadRankedList()
	if err != nil {
		panic(err)
	}

	pps := []float64{200, 300, 400, 500}
	hashByPP := make(map[float64][]string)
	for _, pp := range pps {
		hashByPP[pp] = make([]string, 0)
	}

	hashByStar := make(map[int][]string)
	for hash, entry := range entries {
		for _, diff := range entry.Diffs {
			star := int(math.Trunc(diff.Star))

			if _, ok := hashByStar[star]; !ok {
				hashByStar[star] = make([]string, 0)
			}
			hashByStar[star] = append(hashByStar[star], hash)
		}

		for _, pp := range pps {
			for _, diff := range entry.Diffs {
				if diff.Pp >= pp {
					hashByPP[pp] = append(hashByPP[pp], hash)
					break
				}
			}
		}
	}

	// by star
	for star, hashes := range hashByStar {
		image, err := getImageByStar(imageDir, star)
		if err != nil {
			panic(err)
		}

		of := fmt.Sprintf("%s/ranked_star_%02d.json", outputDir, star)
		if err := writePlaylist(of, fmt.Sprintf("Ranked Songs ★%d", star), "", image, hashes); err != nil {
			panic(err)
		}
	}

	// by performance point
	for pp, hashes := range hashByPP {
		image, err := getImageByPP(imageDir, int(pp))
		if err != nil {
			panic(err)
		}

		of := fmt.Sprintf("%s/ranked_pp_%02d.json", outputDir, int(pp))
		if err := writePlaylist(of, fmt.Sprintf("Ranked Songs %dpp+", int(pp)), "", image, hashes); err != nil {
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

	var entries map[string]RankedEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
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

func writePlaylist(fileName string, title string, description string, image string, hashes []string) error {
	playlist := Playlist{
		Title:       title,
		Author:      "",
		Description: description,
		Image:       image,
		Songs:       make([]PlaylistSong, 0, len(hashes)),
	}

	for _, hash := range hashes {
		playlist.Songs = append(playlist.Songs, PlaylistSong{Hash: hash})
	}

	b, err := json.Marshal(playlist)
	if err != nil {
		return err
	}

	log.Printf("Writing %s...\n", fileName)
	if err := ioutil.WriteFile(fileName, b, 0644); err != nil {
		return err
	}
	return nil
}