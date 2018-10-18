package count

import (
	"github.com/bwmarrin/discordgo"
	"strings"
	"errors"
	"fmt"
	"sort"
	"regexp"
)

type pair struct {
	Key   string
	Value int
}

var (
	sanitizer *regexp.Regexp
	urlPattern *regexp.Regexp
	data      map[string]map[string]int
)

func init() {
	sanitizer = regexp.MustCompile("[^a-z:`]")
	urlPattern = regexp.MustCompile("https?://(www\\.)?[-a-zA-Z0-9@:%._+~#=]{2,256}\\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_+.~#?&//=]*)")
	data = make(map[string]map[string]int)
}

func BuildMessage(m *discordgo.Message) {
	if strings.HasPrefix(m.Content, "!") {
		return
	}
	Build(m.Content, m.Author.Username, !m.Author.Bot)
}

func sanitizeMessage(text string) string {
	text = strings.ToLower(text)

	return text
}

func sanitizeWord(word string) string {
	word = strings.Trim(word, " ")
	word = sanitizer.ReplaceAllString(word, " ")
	word = strings.Trim(word, " ")
	return word
}

func Build(text string, username string, includeInAll bool) {
	text = sanitizeMessage(text)

	if len(text) == 0 {
		return
	}

	words := strings.Split(text, " ")

	for _, word := range words {
		putWord(word, username, includeInAll)
	}
}

func putWord(word string, username string, includeInAll bool) {
	word = sanitizeWord(word)

	if len(word) <= 0 {
		return
	}

	userData := getUserData(username)
	singleUserPutWord(word, userData)
	if includeInAll {
		singleUserPutWord(word, getUserData("all"))
	}
}

func singleUserPutWord(word string, userData map[string]int) {
	if _, ok := userData[word]; ok {
		userData[word]++
	} else {
		userData[word] = 1
	}
}

func getUserData(username string) map[string]int {
	if val, ok := data[username]; ok {
		return val
	}
	userData := make(map[string]int)
	data[username] = userData
	return userData
}

func getSortedSet(set map[string]int) []pair {
	var sortedSet []pair
	for k, v := range set {
		sortedSet = append(sortedSet, pair{k, v})
	}

	sort.Slice(sortedSet, func(i, j int) bool {
		return sortedSet[i].Value > sortedSet[j].Value
	})

	return sortedSet
}

func TopCount(target string) (string, error) {
	if _, ok := data[target]; !ok {
		return "", errors.New(fmt.Sprintf("no such dataset %q", target))
	}

	sortedSet := getSortedSet(data[target])

	if len(sortedSet) > 10 {
		sortedSet = sortedSet[:10]
	}

	var words []string

	for i, p := range sortedSet {
		words = append(words, fmt.Sprintf("%d. %s: %d", i + 1, p.Key, p.Value))
	}

	return strings.Join(words, "\n"), nil
}

func SingleWordCount(target string, word string) (string, error) {
	if _, ok := data[target]; !ok {
		return "", errors.New(fmt.Sprintf("no such dataset %q", target))
	}

	userData := data[target]

	if _, ok := userData[word]; !ok {
		return fmt.Sprintf("user %q has never used that word", target), nil
	}

	if target == "all" {
		target = "everyone"
	}

	return fmt.Sprintf("%s has used the word %q %d times", target, word, userData[word]), nil
}
