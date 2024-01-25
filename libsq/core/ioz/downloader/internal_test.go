package downloader

// Export for testing.

func SetDownloaderDisableCaching(dl *Downloader, disableCaching bool) {
	dl.disableCaching = disableCaching
}
