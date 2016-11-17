package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

// A brief description of program usage
const usage = `
This program accepts one or more URLs as positional arguments and
outputs the number of times the specified target word was found
on each page.

`

// Parses options passed on the command line.
func parseCLI() (word string, numWorkers uint, links []string) {
	// Set the usage message for the cli parser
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] URL1 [URL2 [URL3 ...]]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, usage)
		flag.PrintDefaults()
	}

	// Setup the flags we're looking for
	flag.StringVar(&word, "word", "", "The word to search for.")
	flag.UintVar(&numWorkers, "workers", 1, "The number of workers to use.")

	// Parse the flags
	flag.Parse()

	if word == "" {
		fmt.Fprintf(os.Stderr, "Need a word to process.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if numWorkers < 1 {
		fmt.Fprintf(os.Stderr, "Number of workers must be greater than 0.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Need links to process.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	links = flag.Args()
	return
}

// A Result represents the outcome from counting the occurrence of a
// word on a web page
type Result struct {
	link  string // the path to the page that was inspected
	count uint   // the number of occurrences of the word
	err   error  // an encountered error, if there was one (otherwise nil)
}

// Formats a Result as a string.
func (r Result) String() string {
	return fmt.Sprintf("%s\n\tcount: %d\n\terror: %v", r.link, r.count, r.err)
}

// countOccurrences counts the number of occurrences of `word` in `s`.
func countOccurrences(word string, s io.Reader) (uint, error) {
	// Make a scanner from the s io.Reader, and split by words.
	scanner := bufio.NewScanner(s)
	scanner.Split(bufio.ScanWords)

	var count uint = 0

	for scanner.Scan() {
		// Use scanner.Text() to get the current word.
		// Increment count if the word matches.
		if word == scanner.Text() {
			count++
		}
	}

	// Return error if there is one.
	if err := scanner.Err(); err != nil {
		return count, err
	}
	return count, nil
}

// wordsOnPage reads links from the `links` channel searching for
// occurrences of `word` and sending Results over the `results` channel.
func wordsOnPage(word string, links chan string, results chan Result) {
	// Loop, receiving from links until it is closed.
	for link := range links {
		// Get the link.
		res, err := http.Get(link)

		// Send result with error if there was one.
		if err != nil {
			results <- Result{link, 0, err}
		} else if res.StatusCode != 200 {
			results <- Result{link, 0, errors.New("Did not receive 200 OK")}
		} else {
			count, err := countOccurrences(word, res.Body)
			results <- Result{link, count, err}
		}
	}
}

//Parses CLI args. Spins up workers (goroutines) as specified by the
//user and sets up channels for communication. Sends links over
//channel for processing, closes the channel, then reads all results
//from goroutines.
func main() {
	// Parse options
	word, numWorkers, links := parseCLI()

	// Make the channels for sending/receiving.
	link_chan := make(chan string, len(links))
	result_chan := make(chan Result, len(links))

	// For the number of workers... spin up go routines
	for i := 0; i < int(numWorkers); i++ {
		go wordsOnPage(word, link_chan, result_chan)
	}

	// Send the links for processing.
	for _, link := range links {
		link_chan <- link
	}

	// Close link channel because we are done sending links.
	close(link_chan)

	// Receive results
	for i := 0; i < len(links); i++ {
		fmt.Println(<-result_chan)
	}

}
