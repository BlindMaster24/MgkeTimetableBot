package telegram

import (
	"strings"

	"github.com/mymmrac/telego"
)

type handlerRegistry struct {
	commands     map[string]Command
	commandOrder []Command
	callbacks    map[string]Callback
	textCommands []Command
	scenes       []sceneRoute
}

func (r *handlerRegistry) RegisterCommand(cmd Command) {
	if _, exists := r.commands[cmd.Name()]; !exists {
		r.commandOrder = append(r.commandOrder, cmd)
	}
	r.commands[cmd.Name()] = cmd
}

func (r *handlerRegistry) RegisterTextCommand(cmd Command) {
	r.textCommands = append(r.textCommands, cmd)
}

func (r *handlerRegistry) RegisterCallback(cb Callback) {
	r.callbacks[cb.Prefix()] = cb
}

func (r *handlerRegistry) findCallback(data string) (string, Callback) {
	bestPrefix := ""
	var bestHandler Callback
	for prefix, handler := range r.callbacks {
		if strings.HasPrefix(data, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
			bestHandler = handler
		}
	}
	return bestPrefix, bestHandler
}

func (r *handlerRegistry) commandByName(name string) Command {
	name = strings.ToLower(strings.TrimPrefix(name, "/"))
	for _, cmd := range r.commandOrder {
		if strings.ToLower(strings.TrimPrefix(cmd.Name(), "/")) == name {
			return cmd
		}
	}
	return nil
}

func (r *handlerRegistry) botCommands(includeAdmin bool) []telego.BotCommand {
	cmds := make([]telego.BotCommand, 0, len(r.commandOrder))
	for _, cmd := range r.commandOrder {
		if hidden, ok := cmd.(HiddenCommand); ok && hidden.Hidden() {
			continue
		}

		admin := false
		if ac, ok := cmd.(AdminCommand); ok {
			admin = ac.AdminOnly()
		}
		if admin && !includeAdmin {
			continue
		}

		description := cmd.Description()
		if admin {
			description = "[адм] " + description
		}

		cmds = append(cmds, telego.BotCommand{
			Command:     strings.ToLower(strings.TrimPrefix(cmd.Name(), "/")),
			Description: description,
		})
	}
	return cmds
}
