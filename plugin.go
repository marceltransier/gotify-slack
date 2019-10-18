package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/gotify/plugin-api"
	"github.com/nlopes/slack"
)

// GetGotifyPluginInfo returns gotify plugin info.
func GetGotifyPluginInfo() plugin.Info {
	return plugin.Info{
		ModulePath:  "git.marceltransier.de/gotify-slack",
		Author:      "Marcel Transier",
		Website:     "https://git.marceltransier.de/gotify-slack",
		Description: "Slack push notifications for gotify",
		License:     "MIT",
		Name:        "gotify-slack",
	}
}

// Plugin is the gotify plugin instance.
type Plugin struct {
	enabled    bool
	msgHandler plugin.MessageHandler
	config     *Config
	api        *slack.Client
	rtm        *slack.RTM
	uid        string
	team       string
}

// Config is a user plugin configuration.
type Config struct {
	SlackToken string
}

// Valid checks whether the API token in the config is valid.
func (conf *Config) Valid() bool {
	api := slack.New(conf.SlackToken)
	_, err := api.AuthTest()
	return err == nil
}

// DefaultConfig implements plugin.Configurer.
func (c *Plugin) DefaultConfig() interface{} {
	return &Config{}
}

// ValidateAndSetConfig implements plugin.Configurer.
func (c *Plugin) ValidateAndSetConfig(conf interface{}) error {
	config := conf.(*Config)
	if config.SlackToken == "" {
		return c.stopRTM()
	}
	if !config.Valid() {
		return errors.New("the token is invalid")
	}
	c.config = config
	if !c.enabled {
		return nil
	}
	err := c.stopRTM()
	if err != nil {
		return err
	}
	return c.startRTM()
}

var mentionRe = regexp.MustCompile(`<@[^>]+>`)

func (c *Plugin) startRTM() error {
	c.api = slack.New(c.config.SlackToken)
	atr, err := c.api.AuthTest()
	if err != nil {
		log.Println(err)
		return err
	}
	c.uid = atr.UserID
	c.team = atr.Team
	c.rtm = c.api.NewRTM()
	go c.rtm.ManageConnection()

	for msg := range c.rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			channel, err := c.api.GetConversationInfo(ev.Msg.Channel, true)
			if err != nil {
				log.Println(err)
				continue
			}
			user, err := c.api.GetUserInfo(ev.Msg.User)
			if err != nil {
				log.Println(err)
				continue
			}
			if user.ID == c.uid {
				continue
			}
			title := "Slack | " + c.team + " | "
			if channel.Name != "" {
				title += channel.Name + " | "
			}
			title += user.RealName
			msgtext := mentionRe.ReplaceAllStringFunc(ev.Msg.Text, func(s string) string {
				userid := strings.Trim(s, "<@>")
				user, err := c.api.GetUserInfo(userid)
				if err != nil {
					return "@Error"
				}
				return "@" + user.RealName
			})
			c.msgHandler.SendMessage(plugin.Message{
				Title:    title,
				Message:  msgtext,
				Priority: 5,
			})

		case *slack.InvalidAuthEvent:
			return errors.New("invalid credentials")
		}
	}
	return nil
}

func (c *Plugin) stopRTM() error {
	if c.rtm == nil {
		c.api = nil
		return nil
	}
	return c.rtm.Disconnect()
}

// Enable enables the plugin.
func (c *Plugin) Enable() error {
	if c.config == nil {
		return errors.New("please configure the slack api token first")
	}
	if !c.config.Valid() {
		return errors.New("the slack api token is not valid anymore")
	}
	c.enabled = true
	go c.startRTM()
	return nil
}

// Disable disables the plugin.
func (c *Plugin) Disable() error {
	err := c.stopRTM()
	if err != nil {
		return err
	}
	c.enabled = false
	return nil
}

// GetDisplay implements plugin.Displayer.
func (c *Plugin) GetDisplay(location *url.URL) string {
	return fmt.Sprintf(`
## Status

- Plugin enabled: %t
- Valid API token: %t

Tip: You can get your API token [here](https://api.slack.com/custom-integrations/legacy-tokens).
	`, c.enabled, c.config != nil)
}

// SetMessageHandler implements plugin.Messenger.
func (c *Plugin) SetMessageHandler(h plugin.MessageHandler) {
	c.msgHandler = h
}

// NewGotifyPluginInstance creates a plugin instance for a user context.
func NewGotifyPluginInstance(ctx plugin.UserContext) plugin.Plugin {
	return &Plugin{}
}

func main() {
	panic("this should be built as go plugin")
}
