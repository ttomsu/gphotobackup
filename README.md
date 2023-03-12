# Google Photos Backup Tool

This tool uses the Google Photos API to download all photos and videos to the directory of the user's choice.

My main motivation was to prevent losing all the photos of my children and family should Google ever lock me out of my account.

This tool runs on my NAS drive and runs once per day, looking back at the past N days of uploaded photos.

# Building

```bash
$ go build -o gphotobackup gphotobackup.go
```

# Credentials

1. Enable Google Photos Library API: [https://console.cloud.google.com/apis/library/photoslibrary.googleapis.com](https://console.cloud.google.com/apis/library/photoslibrary.googleapis.com)
2. Create OAuth Client ID (application type "Desktop App")
3. Download the JSON file.

## Login

On a system with a browser available:

```bash
$ gphotobackup login --creds /path/to/downloaded/credentials.json
```

Otherwise, use:

```bash
$ gphotobackup login --browser=false --creds /path/to/downloaded/credentials.json
```

# Sample Usage

```bash
$ gphotobackup print

$ gphotobackup print album --id theBigLongID 

$ gphotobackup backup --albumID theBigLongID

$ gphotobackup backup --start 2021-01-01 --end 2022-01-01 --out /Volumes/GooglePhotosBackup/ --workers 5

$ gphotobackup backup --sinceDays 21
```
