package directory

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/DblK/tinshop/nsp"
	"github.com/DblK/tinshop/repository"
	"github.com/DblK/tinshop/utils"
)

func (src *directorySource) removeGamesWatcherDirectories() {
	log.Println("Removing watcher from all directories")
	if src.watcherDirectories != nil {
		src.watcherDirectories.Close()
	}
}

func (src *directorySource) removeEntriesFromDirectory(directory string) {
	log.Println("removeEntriesFromDirectory", directory)
	for index, game := range src.gameFiles {
		if game.HostType == repository.LocalFile && strings.Contains(game.Path, directory) {
			// Need to remove game
			src.gameFiles = utils.RemoveFileDesc(src.gameFiles, index)

			// Stop watching of directories
			if directory == filepath.Dir(directory) {
				_ = src.watcherDirectories.Remove(filepath.Dir(game.Path))
			}

			// Remove entry from collection
			src.collection.RemoveGame(game.GameID)
		}
	}
}

func (src *directorySource) addDirectoryGame(gameFiles []repository.FileDesc, extension string, size int64, path string) []repository.FileDesc {
	var newGameFiles []repository.FileDesc
	newGameFiles = append(newGameFiles, gameFiles...)

	if extension == ".nsp" || extension == ".nsz" {
		newFile := repository.FileDesc{Size: size, Path: path}
		names := utils.ExtractGameID(path)

		if names.ShortID() != "" {
			newFile.GameID = names.ShortID()
			newFile.GameInfo = names.FullID()
			newFile.HostType = repository.LocalFile

			if src.config.VerifyNSP() {
				valid, errTicket := src.nspCheck(newFile)
				if valid || (errTicket != nil && errTicket.Error() == "TitleDBKey for game "+newFile.GameID+" is not found") {
					newGameFiles = append(newGameFiles, newFile)
				} else {
					log.Println(errTicket)
				}
			} else {
				newGameFiles = append(newGameFiles, newFile)
			}
		} else {
			log.Println("Ignoring file because parsing failed", path)
		}
	}

	return newGameFiles
}

func (src *directorySource) loadGamesDirectory(directory string) error {
	log.Printf("Loading games from directory '%s'...\n", directory)

	var newGameFiles []repository.FileDesc
	// Walk through games directory
	err := filepath.Walk(directory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				extension := filepath.Ext(info.Name())
				newGameFiles = src.addDirectoryGame(newGameFiles, extension, info.Size(), path)
			} else if info.IsDir() {
				if path != directory {
					src.watchDirectory(path)
				} else {
					src.watchDirectory(directory)
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	src.gameFiles = append(src.gameFiles, newGameFiles...)

	// Add all files
	if len(newGameFiles) > 0 {
		src.collection.AddNewGames(newGameFiles)
	}

	return nil
}

func (src *directorySource) nspCheck(file repository.FileDesc) (bool, error) {
	key, err := src.collection.GetKey(file.GameID)
	if err != nil {
		if src.config.DebugTicket() && err.Error() == "TitleDBKey for game "+file.GameID+" is not found" {
			log.Println(err)
		}
		return false, err
	}

	f, err := os.Open(file.Path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	valid, err := nsp.IsTicketValid(f, key, src.config.DebugTicket())
	if err != nil {
		return false, err
	}
	if !valid {
		return false, errors.New("The ticket in '" + file.Path + "' is not valid!")
	}

	return valid, err
}
