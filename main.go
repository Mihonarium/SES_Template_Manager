package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/getsentry/sentry-go"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sesv2"
	"github.com/radovskyb/watcher"
)

type Config struct {
	AWSRegion string     `json:"aws_region"`
	AWSKey    string     `json:"aws_key"`
	AWSSecret string     `json:"aws_secret"`
	Templates []Template `json:"templates"`
	SentryDsn string     `json:"sentry_dsn"`
}

type Template struct {
	TemplateName     string `json:"template_name"`
	SubjectPart      string `json:"subject_part"`
	TextPart         string `json:"text_part"`
	HtmlPartFilePath string `json:"html_part_file_path"`
}

func readConfig(cfgPath string) (Config, error) {
	var cfg Config
	bytes, err := os.ReadFile(cfgPath)
	if capture(err) {
		return cfg, err
	}

	err = json.Unmarshal(bytes, &cfg)
	if capture(err) {
		return cfg, err
	}

	return cfg, nil
}

func readTemplateHTML(filePath string) (string, error) {
	bytes, err := os.ReadFile(filePath)
	if capture(err) {
		return "", err
	}
	return string(bytes), nil
}

const release = "ses_emails@0.1.0"

func updateTemplate(svc *sesv2.SESV2, template Template) {
	fmt.Println("Updating template: " + template.TemplateName)
	htmlPart, err := readTemplateHTML(template.HtmlPartFilePath)
	if capture(err) {
		return
	}
	content := &sesv2.EmailTemplateContent{
		Html:    aws.String(htmlPart),
		Subject: aws.String(template.SubjectPart),
		Text:    aws.String(template.TextPart),
	}
	input := &sesv2.UpdateEmailTemplateInput{
		TemplateContent: content,
		TemplateName:    aws.String(template.TemplateName),
	}
	_, err = svc.UpdateEmailTemplate(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sesv2.ErrCodeNotFoundException:
				// If the template doesn't exist, create it
				createInput := &sesv2.CreateEmailTemplateInput{
					TemplateContent: content,
					TemplateName:    aws.String(template.TemplateName),
				}
				_, err = svc.CreateEmailTemplate(createInput)
				capture(err)
			default:
				// If it's another kind of error, capture it
				capture(aerr)
			}
		} else {
			// Capture non-AWS errors
			capture(err)
		}
	}
}

func updateTemplates(svc *sesv2.SESV2, oldTemplates, newTemplates []Template) {
	oldTemplateNames := make(map[string]struct{})
	for _, oldTemplate := range oldTemplates {
		oldTemplateNames[oldTemplate.TemplateName] = struct{}{}
	}

	for _, newTemplate := range newTemplates {
		_, exists := oldTemplateNames[newTemplate.TemplateName]
		if !exists {
			updateTemplate(svc, newTemplate)
		} else {
			oldTemplate := findTemplate(oldTemplates, newTemplate.TemplateName)
			if !compareTemplates(oldTemplate, &newTemplate) {
				updateTemplate(svc, newTemplate)
			}
		}
	}
}

func main() {
	cfgPath := flag.String("config", "/home/ubuntu/ses_config.json", "Full path to the config file")
	flag.Parse()

	config, err := readConfig(*cfgPath)
	if err != nil {
		panic(err)
	}
	sentryInit(release, config.SentryDsn)
	defer sentry.Recover()

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.AWSRegion),
		Credentials: credentials.NewStaticCredentials(
			config.AWSKey,
			config.AWSSecret,
			"",
		),
	})

	if capture(err) {
		return
	}

	svc := sesv2.New(sess)

	w := watcher.New()

	// Add template files to watch.
	for _, template := range config.Templates {
		w.Add(template.HtmlPartFilePath)
	}
	w.Add(*cfgPath)

	// Update the templates at the start
	updateTemplates(svc, nil, config.Templates)

	go func() {
		oldTemplates := config.Templates
		for {
			select {
			case event := <-w.Event:
				time.Sleep(100 * time.Millisecond)
				if event.Path == *cfgPath {
					fmt.Println("Updating config")
					// Remove old files from the watcher
					for _, template := range config.Templates {
						w.Remove(template.HtmlPartFilePath)
					}
					// Reload the config and continue watching
					config, err = readConfig(*cfgPath)
					if capture(err) {
						break
					}
					for _, template := range config.Templates {
						w.Add(template.HtmlPartFilePath)
					}
					updateTemplates(svc, oldTemplates, config.Templates)
					oldTemplates = config.Templates
				} else {
					for _, template := range config.Templates {
						if event.Path == template.HtmlPartFilePath {
							updateTemplate(svc, template)
						}
					}
				}
			case err := <-w.Error:
				capture(err)
			case <-w.Closed:
				return
			}
		}
	}()

	if err := w.Start(time.Millisecond * 100); err != nil {
		capture(err)
	}
}

// Find template by template name
func findTemplate(templates []Template, name string) *Template {
	for _, template := range templates {
		if template.TemplateName == name {
			return &template
		}
	}
	return nil
}

// Compare if the templates are same
func compareTemplates(template1, template2 *Template) bool {
	if template1 == nil || template2 == nil {
		return false
	}
	return template1.SubjectPart == template2.SubjectPart &&
		template1.TextPart == template2.TextPart &&
		template1.HtmlPartFilePath == template2.HtmlPartFilePath
}
