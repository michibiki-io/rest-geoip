package maxmind

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"rest-geoip/internal/config"
	"rest-geoip/internal/errortypes"
	"rest-geoip/internal/fs"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// DB struct
type DB struct {
	mu sync.RWMutex
	db *maxminddb.Reader
}

var instance *DB
var once sync.Once

// Record captures the data resulting from a query to the maxmind database
type Record struct {
	Country struct {
		IsInEuropeanUnion bool   `maxminddb:"is_in_european_union"`
		ISOCode           string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		AccuracyRadius uint16  `maxminddb:"accuracy_radius"`
		Latitude       float64 `maxminddb:"latitude"`
		Longitude      float64 `maxminddb:"longitude"`
		MetroCode      uint    `maxminddb:"metro_code"`
		TimeZone       string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	Traits struct {
		IsAnonymousProxy    bool `maxminddb:"is_anonymous_proxy"`
		IsSatelliteProvider bool `maxminddb:"is_satellite_provider"`
	} `maxminddb:"traits"`
	Subdivisions []struct {
		IsoCode   string `maxminddb:"iso_code"`
		GeoNameID uint   `maxminddb:"geoname_id"`
	} `maxminddb:"subdivisions"`
	IP string
}

// Open a maxmind database
func (m *DB) Open() error {
	dbLocation := filepath.Join(config.Details().Maxmind.DBLocation, config.Details().Maxmind.DBFileName)
	fmt.Printf("Opening db %s\n", dbLocation)

	_, err := os.Stat(dbLocation)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			e := errortypes.NewErrorDatabaseNotFound(err, dbLocation)
			return fmt.Errorf("maxmind.Open: db not found: %w", e)
		}
		return fmt.Errorf("maxmind.Open: stat failed: %w", err)
	}
	db, err := maxminddb.Open(dbLocation)
	if err != nil {
		return fmt.Errorf("maxmind.Open: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		if err := m.db.Close(); err != nil {
			return fmt.Errorf("maxmind.Open: close previous db: %w", err)
		}
	}
	m.db = db

	return nil
}

// Close a maxmind database
func (m *DB) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db == nil {
		return nil
	}

	if err := m.db.Close(); err != nil {
		return err
	}

	m.db = nil
	return nil
}

func (m *DB) Update() error {
	if config.Details().Maxmind.LicenseKey == "" {
		return fmt.Errorf("Error: can't update database when no license key is set (GOIP_MAXMIND__LICENSE_KEY needs to be set)")
	}

	dbLocation := filepath.Join(config.Details().Maxmind.DBLocation, config.Details().Maxmind.DBFileName)
	_, statErr := os.Stat(dbLocation)
	hadExistingDB := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("maxmind.Update: stat failed: %w", statErr)
	}

	if err := m.Close(); err != nil {
		fmt.Println("Failed to close maxmind database")
		return err
	}
	if err := DownloadAndUpdate(); err != nil {
		fmt.Println("Failed to update maxmind database")
		if hadExistingDB {
			if reopenErr := m.Open(); reopenErr != nil {
				return fmt.Errorf("maxmind.Update: update failed: %w; reopen failed: %w", err, reopenErr)
			}
		}
		return err
	}
	if err := m.Open(); err != nil {
		fmt.Println("Failed to open maxmind database")
		return err
	}

	return nil
}

// Lookup results from a maxmind db lookup
func (m *DB) Lookup(ip net.IP) (Record, error) {
	var record Record

	if ip == nil {
		return record, fmt.Errorf("invalid IP address")
	}

	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()
	if db == nil {
		return record, fmt.Errorf("maxmind database is not open")
	}

	err := db.Lookup(ip, &record)
	if err != nil {
		return record, err
	}

	record.IP = ip.String()
	return record, nil
}

// GetInstance of a maxmindReader
func GetInstance() *DB {
	once.Do(func() {
		instance = &DB{}
	})
	return instance
}

// DownloadAndUpdate the maxmind database
func DownloadAndUpdate() error {
	dbDir := filepath.Clean(config.Details().Maxmind.DBLocation)
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		return fmt.Errorf("maxmind.DownloadAndUpdate: mkdir db dir: %w", err)
	}

	workDir, err := os.MkdirTemp(dbDir, ".maxmind-update-*")
	if err != nil {
		return fmt.Errorf("maxmind.DownloadAndUpdate: create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	dbURL := "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=" + config.Details().Maxmind.LicenseKey + "&suffix=tar.gz"
	md5URL := "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=" + config.Details().Maxmind.LicenseKey + "&suffix=tar.gz.md5"
	dbDest := filepath.Join(workDir, "Geolite.tar.gz")
	md5Dest := filepath.Join(workDir, "Geolite.tar.gz.md5")
	extractDir := filepath.Join(workDir, "extract")

	// Make channels to pass errors in WaitGroup
	downloadErrors := make(chan error, 2)
	wgDone := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	go fs.Download(dbURL, dbDest, &wg, downloadErrors)
	go fs.Download(md5URL, md5Dest, &wg, downloadErrors)

	// wait until WaitGroup is done
	// Sends a signal we need to catch in the select
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	// Wait until either WaitGroup is done or an error is received
	select {
	case <-wgDone:
		break
	case err := <-downloadErrors:
		// close(downloadErrors)
		return err
	}

	if err := fs.VerifyMD5HashFromFile(dbDest, md5Dest); err != nil {
		return err
	}

	// Prepare a reader for extracting the tar.gz
	r, err := os.Open(dbDest) // #nosec G304
	if err != nil {
		return err
	}
	defer r.Close()

	if err := fs.ExtractTarGz(r, extractDir); err != nil {
		return err
	}

	// Move mmdb to the configured database location
	geoCityDBPath, _, err := fs.FindFile(extractDir, `\.mmdb$`)
	if err != nil {
		return err
	}

	destPath := filepath.Join(config.Details().Maxmind.DBLocation, config.Details().Maxmind.DBFileName)
	if err = fs.InstallFileAtomically(geoCityDBPath, destPath, validateDBFile); err != nil {
		return err
	}

	return nil
}

func validateDBFile(path string) error {
	db, err := maxminddb.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("maxmind validate: %w", err)
	}

	return db.Close()
}
