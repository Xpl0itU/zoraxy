package redirection

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"imuslab.com/zoraxy/mod/utils"
)

type RuleTable struct {
	configPath string   //The location where the redirection rules is stored
	rules      sync.Map //Store the redirection rules for this reverse proxy instance
}

type RedirectRules struct {
	RedirectURL      string //The matching URL to redirect
	TargetURL        string //The destination redirection url
	ForwardChildpath bool   //Also redirect the pathname
	StatusCode       int    //Status Code for redirection
}

func NewRuleTable(configPath string) (*RuleTable, error) {
	thisRuleTable := RuleTable{
		rules:      sync.Map{},
		configPath: configPath,
	}
	//Load all the rules from the config path
	if !utils.FileExists(configPath) {
		os.MkdirAll(configPath, 0775)
	}

	// Load all the *.json from the configPath
	files, err := filepath.Glob(filepath.Join(configPath, "*.json"))
	if err != nil {
		return nil, err
	}

	// Parse the json content into RedirectRules
	var rules []*RedirectRules
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		thisRule := RedirectRules{}

		err = json.Unmarshal(b, &thisRule)
		if err != nil {
			continue
		}

		rules = append(rules, &thisRule)
	}

	//Map the rules into the sync map
	for _, rule := range rules {
		log.Printf("Redirection rule added: %s -> %s\n", rule.RedirectURL, rule.TargetURL)
		thisRuleTable.rules.Store(rule.RedirectURL, rule)
	}

	return &thisRuleTable, nil
}

func (t *RuleTable) AddRedirectRule(redirectURL string, destURL string, forwardPathname bool, statusCode int) error {
	// Create a new RedirectRules object with the given parameters
	newRule := &RedirectRules{
		RedirectURL:      redirectURL,
		TargetURL:        destURL,
		ForwardChildpath: forwardPathname,
		StatusCode:       statusCode,
	}

	// Convert the redirectURL to a valid filename by replacing "/" with "-" and "." with "_"
	filename := fmt.Sprintf("%s.json", strings.ReplaceAll(strings.ReplaceAll(redirectURL, "/", "-"), ".", "_"))

	// Create the full file path by joining the t.configPath with the filename
	filepath := path.Join(t.configPath, filename)

	// Create a new file for writing the JSON data
	file, err := os.Create(filepath)
	if err != nil {
		log.Printf("Error creating file %s: %s\n", filepath, err)
		return err
	}
	defer file.Close()

	// Encode the RedirectRules object to JSON and write it to the file
	err = json.NewEncoder(file).Encode(newRule)
	if err != nil {
		log.Printf("Error encoding JSON to file %s: %s\n", filepath, err)
		return err
	}

	// Store the RedirectRules object in the sync.Map
	t.rules.Store(redirectURL, newRule)

	return nil
}

func (t *RuleTable) DeleteRedirectRule(redirectURL string) error {
	// Convert the redirectURL to a valid filename by replacing "/" with "-" and "." with "_"
	filename := fmt.Sprintf("%s.json", strings.ReplaceAll(strings.ReplaceAll(redirectURL, "/", "-"), ".", "_"))

	// Create the full file path by joining the t.configPath with the filename
	filepath := path.Join(t.configPath, filename)

	// Check if the file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to delete
	}

	// Delete the file
	if err := os.Remove(filepath); err != nil {
		log.Printf("Error deleting file %s: %s\n", filepath, err)
		return err
	}

	// Delete the key-value pair from the sync.Map
	t.rules.Delete(redirectURL)

	return nil
}

// Get a list of all the redirection rules
func (t *RuleTable) GetAllRedirectRules() []*RedirectRules {
	rules := []*RedirectRules{}
	t.rules.Range(func(key, value interface{}) bool {
		r, ok := value.(*RedirectRules)
		if ok {
			rules = append(rules, r)
		}
		return true
	})
	return rules
}

// Check if a given request URL matched any of the redirection rule
func (t *RuleTable) MatchRedirectRule(requestedURL string) *RedirectRules {
	// Iterate through all the keys in the rules map
	var targetRedirectionRule *RedirectRules = nil
	var maxMatch int = 0

	t.rules.Range(func(key interface{}, value interface{}) bool {
		// Check if the requested URL starts with the key as a prefix
		if strings.HasPrefix(requestedURL, key.(string)) {
			// This request URL matched the domain
			if len(key.(string)) > maxMatch {
				maxMatch = len(key.(string))
				targetRedirectionRule = value.(*RedirectRules)
			}
		}
		return true
	})

	return targetRedirectionRule
}
