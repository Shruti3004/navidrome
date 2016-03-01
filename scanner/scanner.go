package scanner

import (
	"github.com/astaxie/beego"
	"github.com/deluan/gosonic/repositories"
	"github.com/deluan/gosonic/models"
	"strings"
"github.com/deluan/gosonic/utils"
	"github.com/deluan/gosonic/consts"
	"time"
	"fmt"
)

type Scanner interface {
	LoadFolder(path string) []Track
}

type tempIndex map[string]models.ArtistInfo

// TODO Implement a flag 'isScanning'.
func StartImport() {
	go doImport(beego.AppConfig.String("musicFolder"), &ItunesScanner{})
}

func doImport(mediaFolder string, scanner Scanner) {
	beego.Info("Starting iTunes import from:", mediaFolder)
	files := scanner.LoadFolder(mediaFolder)
	importLibrary(files)
	beego.Info("Finished importing", len(files), "files")
}

func importLibrary(files []Track) (err error){
	indexGroups := utils.ParseIndexGroups(beego.AppConfig.String("indexGroups"))
	mfRepo := repositories.NewMediaFileRepository()
	albumRepo := repositories.NewAlbumRepository()
	artistRepo := repositories.NewArtistRepository()
	var artistIndex = make(map[string]tempIndex)

	for _, t := range files {
		mf, album, artist := parseTrack(&t)
		persist(mfRepo, mf, albumRepo, album, artistRepo, artist)
		collectIndex(indexGroups, artist, artistIndex)
	}

	if err = saveIndex(artistIndex); err != nil {
		beego.Error(err)
	}

	c, _ := artistRepo.CountAll()
	beego.Info("Total Artists in database:", c)
	c, _ = albumRepo.CountAll()
	beego.Info("Total Albums in database:", c)
	c, _ = mfRepo.CountAll()
	beego.Info("Total MediaFiles in database:", c)

	if err == nil {
		propertyRepo := repositories.NewPropertyRepository()
		millis := time.Now().UnixNano() / 1000000
		propertyRepo.Put(consts.LastScan, fmt.Sprint(millis))
		beego.Info("LastScan timestamp:", millis)
	}

	return err
}

func parseTrack(t *Track) (*models.MediaFile, *models.Album, *models.Artist) {
	mf := &models.MediaFile{
		Id: t.Id,
		Album: t.Album,
		Artist: t.Artist,
		AlbumArtist: t.AlbumArtist,
		Title: t.Title,
		Compilation: t.Compilation,
		Path: t.Path,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	album := &models.Album{
		Name: t.Album,
		Year: t.Year,
		Compilation: t.Compilation,
	}

	artist := &models.Artist{
		Name: t.RealArtist(),
	}

	return mf, album, artist
}

func persist(mfRepo *repositories.MediaFile, mf *models.MediaFile, albumRepo *repositories.Album, album *models.Album, artistRepo *repositories.Artist, artist *models.Artist) {
	if err := artistRepo.Put(artist); err != nil {
		beego.Error(err)
	}

	album.ArtistId = artist.Id
	if err := albumRepo.Put(album); err != nil {
		beego.Error(err)
	}

	mf.AlbumId = album.Id
	if err := mfRepo.Put(mf); err != nil {
		beego.Error(err)
	}
}

func collectIndex(ig utils.IndexGroups, a *models.Artist, artistIndex map[string]tempIndex) {
	name := a.Name
	indexName := strings.ToLower(utils.NoArticle(name))
	if indexName == "" {
		return
	}
	group := findGroup(ig, indexName)
	artists := artistIndex[group]
	if artists == nil {
		artists = make(tempIndex)
		artistIndex[group] = artists
	}
	artists[indexName] = models.ArtistInfo{ArtistId: a.Id, Artist: a.Name}
}

func findGroup(ig utils.IndexGroups, name string) string {
	for k, v := range ig {
		key := strings.ToLower(k)
		if strings.HasPrefix(name, key) {
			return v
		}
	}
	return "#"
}

func saveIndex(artistIndex map[string]tempIndex) error {
	idxRepo := repositories.NewArtistIndexRepository()

	for k, temp := range artistIndex {
		idx := &models.ArtistIndex{Id: k}
		for _, v := range temp {
			idx.Artists = append(idx.Artists, v)
		}
		err := idxRepo.Put(idx)
		if err != nil {
			return err
		}
	}

	return nil
}