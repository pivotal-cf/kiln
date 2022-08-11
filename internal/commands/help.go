package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pivotal-cf/jhanda"
)

type helpData struct {
	// input
	Title         string
	Description   string
	Usage         string
	GlobalFlags   []string
	ArgumentsName string
	ArgumentLines []string
}

func (tc helpData) String() string {
	var sb strings.Builder

	if tc.Title != "kiln" {
		sb.WriteString("\n")
		sb.WriteString(tc.Title)
		sb.WriteString("\n\n")
	}
	if tc.Description != "" {
		sb.WriteString(tc.Description)
		sb.WriteString("\n\n")
	}
	if tc.Usage != "" {
		sb.WriteString("Usage: ")
		sb.WriteString(tc.Usage)
		sb.WriteString("\n")
	}
	if len(tc.GlobalFlags) > 0 {
		for _, flag := range tc.GlobalFlags {
			sb.WriteString("  ")
			sb.WriteString(flag)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if tc.ArgumentsName != "" {
		sb.WriteString(tc.ArgumentsName)
		sb.WriteString("\n")
	}

	for _, line := range tc.ArgumentLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	return sb.String()
}

type Help struct {
	output   io.Writer
	flags    string
	commands jhanda.CommandSet
	groups   map[string][]string
}

func NewHelp(output io.Writer, flags string, commands jhanda.CommandSet, groups map[string][]string) Help {
	return Help{
		output:   output,
		flags:    flags,
		commands: commands,
		groups:   groups,
	}
}

func (h Help) Execute(args []string) error {
	globalFlags := strings.Split(h.flags, "\n")

	var data helpData
	if len(args) == 0 {
		data = h.buildGlobalContext()
	} else {
		var err error
		data, err = h.buildCommandContext(args[0])
		if err != nil {
			return err
		}
	}
	data.GlobalFlags = globalFlags

	_, err := fmt.Fprintf(h.output, "%s", data)
	return err
}

func (h Help) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command prints helpful usage information.",
		ShortDescription: "prints this usage information",
	}
}

func (h Help) buildGlobalContext() helpData {
	var commands []string

	for groupName, groupCommandNames := range h.groups {
		if len(groupCommandNames) == 0 {
			continue
		}
		names := groupCommandNames
		maxLength := maxLen(names)
		sort.Strings(names)
		commands = append(commands, fmt.Sprintf("%s:", groupName))

		for _, name := range names {
			command := h.commands[name]
			name = padCommand(name, " ", maxLength)
			commands = append(commands, fmt.Sprintf("  %s  %s", name, command.Usage().ShortDescription))
		}

		commands = append(commands, "")
	}

	commands = commands[:len(commands)-1]

	result := helpData{
		Title:         "kiln",
		Description:   "kiln helps you build ops manager compatible tiles",
		Usage:         "kiln [options] <command> [<args>]",
		ArgumentLines: commands,
	}

	return result
}

func (h Help) buildCommandContext(command string) (helpData, error) {
	usage, err := h.commands.Usage(command)
	if err != nil {
		return helpData{}, err
	}

	var (
		flagList        []string
		argsPlaceholder string
	)
	if usage.Flags != nil {
		flagUsage, err := jhanda.PrintUsage(usage.Flags)
		if err != nil {
			return helpData{}, err
		}

		for _, flag := range strings.Split(flagUsage, "\n") {
			if flag != "" {
				flagList = append(flagList, "  "+flag)
			}
		}

		if len(flagList) != 0 {
			argsPlaceholder = " [<args>]"
		}
	}

	return helpData{
		Title:         fmt.Sprintf("kiln %s", command),
		Description:   usage.Description,
		Usage:         fmt.Sprintf("kiln [options] %s%s", command, argsPlaceholder),
		ArgumentsName: "Flags",
		ArgumentLines: flagList,
	}, nil
}

func padCommand(str, pad string, length int) string {
	return str + strings.Repeat(pad, length-len(str))
}

func maxLen(slice []string) int {
	var max int
	for _, e := range slice {
		if len(e) > max {
			max = len(e)
		}
	}
	return max
}
