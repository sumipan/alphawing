package models

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/coopernurse/gorp"
)

type BundlePlatformType int

const (
	BundlePlatformTypeAndroid BundlePlatformType = 1 + iota
	BundlePlatformTypeIOS
)

func (platformType BundlePlatformType) Extention() BundleFileExtension {
	var ext BundleFileExtension
	if platformType == BundlePlatformTypeAndroid {
		ext = BundleFileExtensionAndroid
	} else if platformType == BundlePlatformTypeIOS {
		ext = BundleFileExtensionIOS
	}
	return ext
}

type BundleFileExtension string

const (
	BundleFileExtensionAndroid BundleFileExtension = ".apk"
	BundleFileExtensionIOS     BundleFileExtension = ".ipa"
)

func (ext BundleFileExtension) IsValid() bool {
	var ok bool
	if ext == BundleFileExtensionAndroid {
		ok = true
	} else if ext == BundleFileExtensionIOS {
		ok = true
	}
	return ok
}

func (ext BundleFileExtension) PlatformType() BundlePlatformType {
	var platformType BundlePlatformType
	if ext == BundleFileExtensionAndroid {
		platformType = BundlePlatformTypeAndroid
	} else if ext == BundleFileExtensionIOS {
		platformType = BundlePlatformTypeIOS
	}
	return platformType
}

type Bundle struct {
	Id            int                `db:"id"`
	AppId         int                `db:"app_id"`
	FileId        string             `db:"file_id"`
	PlatformType  BundlePlatformType `db:"platform_type"`
	BundleVersion string             `db:"bundle_version"`
	Revision      int                `db:"revision"`
	Description   string             `db:"description"`
	CreatedAt     time.Time          `db:"created_at"`
	UpdatedAt     time.Time          `db:"updated_at"`

	BundleInfo *BundleInfo `db:"-"`
	File       *os.File    `db:"-"`
	FileName   string      `db:"-"`
}

type BundleJsonResponse struct {
	FileId     string `json:"file_id"`
	Version    string `json:"version"`
	Revision   int    `json:"revision"`
	InstallUrl string `json:"install_url"`
	QrCodeUrl  string `json:"qr_code_url"`
}

func (bundle *Bundle) JsonResponse(ub UriBuilder) (*BundleJsonResponse, error) {
	installUrl, err := ub.UriFor(fmt.Sprintf("bundle/%d/download", bundle.Id))
	if err != nil {
		return nil, err
	}
	qrCodeUrl, err := ub.UriFor(fmt.Sprintf("bundle/%d", bundle.Id))
	if err != nil {
		return nil, err
	}

	return &BundleJsonResponse{
		FileId:     bundle.FileId,
		Version:    bundle.BundleVersion,
		Revision:   bundle.Revision,
		InstallUrl: installUrl.String(),
		QrCodeUrl:  qrCodeUrl.String(),
	}, nil
}

func (bundle *Bundle) Plist(txn gorp.SqlExecutor, ipaUrl *url.URL) (*Plist, error) {
	app, err := bundle.App(txn)
	if err != nil {
		return nil, err
	}

	return NewPlist(app.Title, bundle.BundleVersion, ipaUrl.String()), nil
}

func (bundle *Bundle) PlistReader(txn gorp.SqlExecutor, ipaUrl *url.URL) (io.Reader, error) {
	p, err := bundle.Plist(txn, ipaUrl)
	if err != nil {
		return nil, err
	}

	return p.Reader()
}

func (bundle *Bundle) BuildFileName() string {
	return fmt.Sprintf(
		"app_%d_ver_%s_rev_%d%s",
		bundle.AppId,
		bundle.BundleInfo.Version,
		bundle.Revision,
		bundle.PlatformType.Extention(),
	)
}

func (bundle *Bundle) IsApk() bool {
	var ok bool
	if bundle.PlatformType == BundlePlatformTypeAndroid {
		ok = true
	}
	return ok
}

func (bundle *Bundle) IsIpa() bool {
	var ok bool
	if bundle.PlatformType == BundlePlatformTypeIOS {
		ok = true
	}
	return ok
}

func (bundle *Bundle) App(txn gorp.SqlExecutor) (*App, error) {
	app, err := txn.Get(App{}, bundle.AppId)
	if err != nil {
		return nil, err
	}
	return app.(*App), nil
}

func (bundle *Bundle) PreInsert(s gorp.SqlExecutor) error {
	bundle.BundleVersion = bundle.BundleInfo.Version
	bundle.CreatedAt = time.Now()
	bundle.UpdatedAt = bundle.CreatedAt
	return nil
}

func (bundle *Bundle) PreUpdate(s gorp.SqlExecutor) error {
	bundle.UpdatedAt = time.Now()
	return nil
}

func (bundle *Bundle) Save(txn gorp.SqlExecutor) error {
	return txn.Insert(bundle)
}

func (bundle *Bundle) Update(txn gorp.SqlExecutor) error {
	current, err := GetBundle(txn, bundle.Id)
	if err != nil {
		return err
	}

	current.Description = bundle.Description
	if bundle.FileId != "" {
		current.FileId = bundle.FileId
	}

	_, err = txn.Update(current)
	return err
}

func (bundle *Bundle) DeleteFromDB(txn gorp.SqlExecutor) error {
	_, err := txn.Delete(bundle)
	return err
}

func (bundle *Bundle) DeleteFromGoogleDrive(s *GoogleService) error {
	return s.DeleteFile(bundle.FileId)
}

func (bundle *Bundle) Delete(txn gorp.SqlExecutor, s *GoogleService) error {
	if err := bundle.DeleteFromDB(txn); err != nil {
		return err
	}
	if err := bundle.DeleteFromGoogleDrive(s); err != nil {
		return err
	}
	return nil
}

func CreateBundle(txn gorp.SqlExecutor, bundle *Bundle) error {
	return txn.Insert(bundle)
}

func GetBundle(txn gorp.SqlExecutor, id int) (*Bundle, error) {
	bundle, err := txn.Get(Bundle{}, id)
	if err != nil {
		return nil, err
	}
	return bundle.(*Bundle), nil
}

func GetBundleByFileId(txn *gorp.Transaction, fileId string) (*Bundle, error) {
	var bundle *Bundle
	err := txn.SelectOne(&bundle, "SELECT * FROM bundle WHERE file_id = ?", fileId)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}
