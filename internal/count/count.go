package count

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

type set struct {
	includeAll bool
	counts     map[string]int
}

type pair struct {
	Key   string
	Value int
}

var (
	sanitizer  *regexp.Regexp
	urlPattern *regexp.Regexp
	data       map[string]set
)

func init() {
	sanitizer = regexp.MustCompile("[`\\[\\]{}()?'\",.&]")
	urlPattern = regexp.MustCompile("https?://(www\\.)?[-a-zA-Z0-9@:%._+~#=]{2,256}\\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_+.~#?&//=]*)")
	data = make(map[string]set)
}

func BuildMessage(m *discordgo.Message) {
	if strings.HasPrefix(m.Content, "!") {
		return
	}
	Build(m.Content, m.Author.Username, !m.Author.Bot)
}

func sanitizeMessage(text string) string {
	text = strings.ToLower(text)
	text = urlPattern.ReplaceAllString(text, " ")
	return text
}

func sanitizeWord(word string) string {
	word = strings.ToLower(word)
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

	if len(word) == 0 {
		return
	}

	if !setExists(username) {
		createSet(username, includeInAll)
	}

	singleUserPutWord(word, data[username])
	if includeInAll {
		if !setExists("all") {
			createSet("all", false)
		}
		singleUserPutWord(word, data["all"])
	}
}

func singleUserPutWord(word string, userData set) {
	if _, ok := userData.counts[word]; ok {
		userData.counts[word]++
	} else {
		userData.counts[word] = 1
	}
}

func createSet(name string, includeAll bool) {
	dataSet := set{
		counts:     make(map[string]int),
		includeAll: includeAll,
	}
	data[name] = dataSet
}

func setExists(name string) bool {
	_, ok := data[name]
	return ok
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

func wordRankedPerUser(word string) (string, error) {
	log.Info("wordRankedPerUser called with word ", word)
	rankSet := make(map[string]int)
	for setName, set := range data {
		if set.includeAll {
			if c, ok := set.counts[word]; ok {
				rankSet[setName] = c
			}
		}
	}

	sortedSet := getSortedSet(rankSet)

	if len(sortedSet) > 5 {
		sortedSet = sortedSet[:5]
	}

	var userWordCounts []string
	for i, p := range sortedSet {
		userWordCounts = append(userWordCounts, fmt.Sprintf("%d. %s: %d", i+1, p.Key, p.Value))
	}

	if len(userWordCounts) == 0 {
		return "", fmt.Errorf("no one has said %s", word)
	}

	return strings.Join(userWordCounts, "\n"), nil
}

// TopCount returns the top words for a user
func TopCount(target string) (string, error) {
	if !setExists(target) {
		return wordRankedPerUser(target)
	}

	sortedSet := getSortedSet(data[target].counts)

	if len(sortedSet) > 5 {
		sortedSet = sortedSet[:5]
	}

	var words []string

	for i, p := range sortedSet {
		words = append(words, fmt.Sprintf("%d. %s: %d", i+1, p.Key, p.Value))
	}

	if len(words) == 0 {
		return "", fmt.Errorf("target %s has no words", target)
	}

	return strings.Join(words, "\n"), nil
}

// SingleWordCount returns the count of a word from a target dataset
func SingleWordCount(target string, word string) (string, error) {
	if _, ok := data[target]; !ok {
		return "", fmt.Errorf("no such dataset %q", target)
	}

	userData := data[target]

	log.Info("word", word)

	word = sanitizeWord(word)
	log.Info("sanitized word ", word)
	if len(word) == 0 {
		return "", errors.New("word contains only sanitized chars")
	}

	if _, ok := userData.counts[word]; !ok {
		return fmt.Sprintf("user %q has never said that", target), nil
	}

	if target == "all" {
		target = "everyone"
	}

	return fmt.Sprintf("%s has said %s %d times", target, word, userData.counts[word]), nil
}
