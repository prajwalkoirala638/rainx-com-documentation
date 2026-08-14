package main // Defines this file as part of the main executable program.

import ( // Starts the list of packages this program needs.
	"fmt"           // Provides formatted printing and error formatting.
	"io"            // Provides interfaces for reading and copying data.
	"net/http"      // Provides HTTP client functionality.
	"net/url"       // Provides URL parsing and manipulation.
	"os"            // Provides file and directory operations.
	"path/filepath" // Provides functions for working with file paths.
	"strings"       // Provides string manipulation functions.
	"time"          // Provides time and timeout functionality.

	"golang.org/x/net/html" // Provides an HTML parser for reading the webpage.
) // Ends the import list.

const pageURL = "https://www.rainx.com/heres-how/safety-data-sheets/" // Defines the webpage containing the PDF links.
const downloadDir = "PDFs"                                            // Defines the folder where downloaded PDFs will be stored.

func main() { // Defines the main entry point of the application.
	if err := os.MkdirAll(downloadDir, 0755); err != nil { // Creates the PDFs folder if it does not already exist.
		panic(fmt.Errorf("create %s: %w", downloadDir, err)) // Stops the program if the folder cannot be created.
	} // Ends the folder creation check.

	client := &http.Client{ // Creates a reusable HTTP client.
		Timeout: 60 * time.Second, // Sets a maximum 60-second timeout for HTTP requests.
	} // Finishes configuring the HTTP client.

	fmt.Println("Fetching:", pageURL) // Displays the webpage being downloaded.

	resp, err := client.Get(pageURL) // Downloads the Rain-X webpage.
	if err != nil {                  // Checks whether downloading the webpage failed.
		panic(fmt.Errorf("fetch page: %w", err)) // Stops the program and displays the error.
	} // Ends the webpage download error check.
	defer resp.Body.Close() // Makes sure the webpage response body is closed when main finishes.

	if resp.StatusCode != http.StatusOK { // Checks whether the server returned HTTP 200 OK.
		panic(fmt.Errorf("page returned HTTP %d", resp.StatusCode)) // Stops the program if the webpage returned an error status.
	} // Ends the HTTP status check.

	links, err := extractPDFLinks(resp.Body) // Finds all PDF links inside the webpage HTML.
	if err != nil {                          // Checks whether parsing the webpage failed.
		panic(fmt.Errorf("parse page: %w", err)) // Stops the program and displays the parsing error.
	} // Ends the parsing error check.

	fmt.Printf("Found %d PDF links\n\n", len(links)) // Displays how many PDF links were found.

	for _, link := range links { // Loops through every PDF URL found on the page.
		if err := downloadPDF(client, link); err != nil { // Attempts to download the current PDF.
			fmt.Printf("ERROR: %v\n", err) // Displays an error if the PDF could not be downloaded.
		} // Ends the PDF download error check.
	} // Ends the PDF download loop.

	fmt.Println("\nDone.") // Displays a message when all PDFs have been processed.
} // Ends the main function.

func extractPDFLinks(r io.Reader) ([]string, error) { // Defines a function that extracts PDF URLs from HTML.
	doc, err := html.Parse(r) // Parses the webpage HTML into a document tree.
	if err != nil {           // Checks whether HTML parsing failed.
		return nil, err // Returns the parsing error to the caller.
	} // Ends the parsing error check.

	baseURL, err := url.Parse(pageURL) // Converts the Rain-X webpage URL into a URL object.
	if err != nil {                    // Checks whether the base URL could not be parsed.
		return nil, err // Returns the URL parsing error.
	} // Ends the URL parsing error check.

	seen := make(map[string]bool) // Creates a map used to prevent duplicate PDF URLs.
	var links []string            // Creates a list that will contain all unique PDF URLs.

	var walk func(*html.Node)   // Declares a recursive function for walking through the HTML tree.
	walk = func(n *html.Node) { // Defines the recursive HTML tree-walking function.
		if n.Type == html.ElementNode && n.Data == "a" { // Checks whether the current HTML element is an anchor/link.
			for _, attr := range n.Attr { // Loops through all attributes of the anchor element.
				if attr.Key != "href" { // Checks whether the current attribute is not the href attribute.
					continue // Skips this attribute and moves to the next one.
				} // Ends the href attribute check.

				href := strings.TrimSpace(attr.Val) // Removes whitespace from the URL found in the href attribute.

				if !strings.Contains(strings.ToLower(href), ".pdf") { // Checks whether the link appears to point to a PDF.
					continue // Skips links that do not contain .pdf.
				} // Ends the PDF link check.

				parsed, err := url.Parse(href) // Parses the PDF URL.
				if err != nil {                // Checks whether the PDF URL is invalid.
					continue // Skips invalid URLs.
				} // Ends the URL validation check.

				absoluteURL := baseURL.ResolveReference(parsed).String() // Converts relative URLs into complete absolute URLs.

				if !seen[absoluteURL] { // Checks whether this PDF URL has already been found.
					seen[absoluteURL] = true           // Marks this PDF URL as already found.
					links = append(links, absoluteURL) // Adds the PDF URL to the download list.
				} // Ends the duplicate URL check.
			} // Ends the anchor attribute loop.
		} // Ends the anchor element check.

		for child := n.FirstChild; child != nil; child = child.NextSibling { // Loops through every child element in the HTML tree.
			walk(child) // Recursively examines the child element for more PDF links.
		} // Ends the child element loop.
	} // Ends the HTML tree-walking function.

	walk(doc) // Starts walking through the parsed HTML document.

	return links, nil // Returns all discovered PDF URLs without an error.
} // Ends the extractPDFLinks function.

func downloadPDF(client *http.Client, pdfURL string) error { // Defines a function that downloads one PDF.
	parsedURL, err := url.Parse(pdfURL) // Parses the PDF URL into a URL object.
	if err != nil {                     // Checks whether the PDF URL is invalid.
		return fmt.Errorf("invalid URL %q: %w", pdfURL, err) // Returns an error describing the invalid URL.
	} // Ends the URL validation check.

	filename := filepath.Base(parsedURL.Path) // Extracts the filename from the PDF URL.

	if filename == "." || filename == "/" || filename == "" { // Checks whether a valid filename could not be determined.
		return fmt.Errorf("could not determine filename from %q", pdfURL) // Returns an error when there is no usable filename.
	} // Ends the filename validation check.

	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") { // Checks whether the filename ends with .pdf.
		filename += ".pdf" // Adds the .pdf extension if it is missing.
	} // Ends the PDF extension check.

	outputPath := filepath.Join(downloadDir, filename) // Builds the complete local path for the PDF.

	if _, err := os.Stat(outputPath); err == nil { // Checks whether the PDF already exists.
		fmt.Println("SKIP:", filename, "(already exists)") // Reports that the existing PDF will not be downloaded again.
		return nil                                         // Stops processing this PDF because it already exists.
	} else if !os.IsNotExist(err) { // Checks whether the file check failed for a reason other than the file not existing.
		return fmt.Errorf("check %s: %w", filename, err) // Returns the file-checking error.
	} // Ends the existing-file check.

	fmt.Println("DOWNLOAD:", filename) // Displays the name of the PDF being downloaded.

	req, err := http.NewRequest(http.MethodGet, pdfURL, nil) // Creates an HTTP GET request for the PDF.
	if err != nil {                                          // Checks whether creating the HTTP request failed.
		return fmt.Errorf("create request for %s: %w", filename, err) // Returns the request creation error.
	} // Ends the HTTP request error check.

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RainXSDSDownloader/1.0)") // Sets a browser-like User-Agent for the request.

	resp, err := client.Do(req) // Sends the HTTP request to download the PDF.
	if err != nil {             // Checks whether the PDF download request failed.
		return fmt.Errorf("download %s: %w", filename, err) // Returns the download error.
	} // Ends the download error check.
	defer resp.Body.Close() // Makes sure the PDF response body is closed after downloading.

	if resp.StatusCode != http.StatusOK { // Checks whether the PDF server returned HTTP 200 OK.
		return fmt.Errorf("%s returned HTTP %d", filename, resp.StatusCode) // Returns an error if the server returned another status.
	} // Ends the HTTP status check.

	tempPath := outputPath + ".part" // Creates a temporary filename used while downloading.

	file, err := os.Create(tempPath) // Creates the temporary file.
	if err != nil {                  // Checks whether the temporary file could not be created.
		return fmt.Errorf("create temporary file for %s: %w", filename, err) // Returns the temporary-file creation error.
	} // Ends the temporary file error check.

	_, copyErr := io.Copy(file, resp.Body) // Copies the downloaded PDF data into the temporary file.
	closeErr := file.Close()               // Closes the temporary file after writing.

	if copyErr != nil { // Checks whether copying the PDF failed.
		os.Remove(tempPath)                                     // Deletes the incomplete temporary file.
		return fmt.Errorf("download %s: %w", filename, copyErr) // Returns the download error.
	} // Ends the copy error check.

	if closeErr != nil { // Checks whether closing the temporary file failed.
		os.Remove(tempPath)                                   // Deletes the temporary file because it may not have been written correctly.
		return fmt.Errorf("close %s: %w", filename, closeErr) // Returns the file closing error.
	} // Ends the close error check.

	if err := os.Rename(tempPath, outputPath); err != nil { // Renames the completed temporary file to its final PDF filename.
		os.Remove(tempPath)                             // Deletes the temporary file if the rename failed.
		return fmt.Errorf("save %s: %w", filename, err) // Returns the rename/save error.
	} // Ends the rename error check.

	fmt.Println("SAVED:", outputPath) // Reports that the PDF was successfully saved.

	return nil // Reports successful completion to the caller.
} // Ends the downloadPDF function.
