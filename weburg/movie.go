package weburg

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackpal/bencode-go"
)

const (
	MovieSourceURL = "http://weburg.net/ajax/download/movie?obj_id=%s"
)

var (
	TorrentRegexp       = regexp.MustCompile(`href\=\"([^\"]*)\"`)
	TorrentNameRegexp   = regexp.MustCompile(`class\=\"objects\_\_name\"\>([^\<]*)\<`)
	TorrentSizeRegexp   = regexp.MustCompile(`class\=\"objects\-metric\_\_size\"\>([^\<]*)\<`)
	WeburgMovieIDRegexp = regexp.MustCompile(`weburg.net/movies/info/([0-9]+)`)
)

type Movie struct {
	DownloadURL string `json:"download_url"`
	Size        string `json:"size"`
	Name        string `json:"name"`
	torrentURL  string `json:"-"`
}

type MetaInfo struct {
	UrlList []string "url-list"
}

type MovieService struct {
	Client *Client
	movies []*Movie
}

func (m *MovieService) RawSources(movieID string) (string, error) {
	u := fmt.Sprintf(MovieSourceURL, movieID)
	req, err := m.Client.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	body, err := m.Client.Do(req)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (m *MovieService) ParseBody(body string) error {
	torrentData := TorrentRegexp.FindAllStringSubmatch(body, -1)
	if len(torrentData) == 0 {
		return errors.New("Nothing found")
	}
	torrentNameData := TorrentNameRegexp.FindAllStringSubmatch(body, -1)
	if len(torrentNameData) == 0 {
		return errors.New("Nothing found")
	}
	torrentSizeData := TorrentSizeRegexp.FindAllStringSubmatch(body, -1)
	if len(torrentSizeData) == 0 {
		return errors.New("Nothing found")
	}
	for id, collection := range torrentData {
		movie := &Movie{
			torrentURL: collection[1],
			Name:       torrentNameData[id][1],
			Size:       torrentSizeData[id][1],
		}
		m.movies = append(m.movies, movie)
	}
	return nil
}

func (m *MovieService) ParseTorrentURL() error {
	for id, movie := range m.movies {
		meta, err := m.GetTorrentMeta(movie.torrentURL)
		if err != nil || len(meta.UrlList) == 0 {
			continue
		}
		movie.DownloadURL = meta.UrlList[0]
		m.movies[id] = movie
	}
	return nil
}

func (m *MovieService) GetTorrentMeta(url string) (*MetaInfo, error) {
	meta := &MetaInfo{}
	req, err := m.Client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	body, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	err = bencode.Unmarshal(bytes.NewReader(body), &meta)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

func getMovieIDFromLink(rawurl string) (string, error) {
	idData := WeburgMovieIDRegexp.FindStringSubmatch(rawurl)
	if len(idData) < 1 {
		return "", errors.New("Get non Weburg link:" + rawurl)
	}
	return idData[1], nil
}

func (m *MovieService) Info(rawurl string) ([]*Movie, error) {
	m.movies = []*Movie{}
	movieID, err := getMovieIDFromLink(rawurl)
	if err != nil {
		return nil, err
	}
	body, err := m.RawSources(movieID)
	if err != nil {
		return nil, err
	}
	err = m.ParseBody(body)
	if err != nil {
		return nil, err
	}
	err = m.ParseTorrentURL()
	if err != nil {
		return nil, err
	}
	return m.movies, nil
}
