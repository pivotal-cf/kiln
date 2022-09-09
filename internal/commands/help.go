package commands

import (
	"fmt"
	"golang.org/x/exp/maps"
	"io"
	"reflect"
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
	output     io.Writer
	flags      string
	commands   jhanda.CommandSet
	groupOrder []string
	groups     map[string][]string
}

func NewHelp(output io.Writer, flags string, commands jhanda.CommandSet, groupOrder []string, groups map[string][]string) Help {
	if len(groupOrder) == 0 {
		groupNames := maps.Keys(groups)
		sort.Strings(groupNames)
		groupOrder = groupNames
	}
	return Help{
		output:     output,
		flags:      flags,
		commands:   commands,
		groupOrder: groupOrder,
		groups:     groups,
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

	for _, groupName := range h.groupOrder {
		groupCommandNames := h.groups[groupName]
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
		flagUsage, err := printUsage(usage.Flags)
		if err != nil {
			return helpData{}, err
		}

		for _, flag := range strings.Split(flagUsage, "\n") {
			if flag == "" {
				continue
			}
			flagList = append(flagList, "  "+flag)
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

func printUsage(receiver interface{}) (string, error) {
	v := reflect.ValueOf(receiver)
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("unexpected pointer to non-struct type %s", t.Kind())
	}

	fields := getFields(t)

	var usage []string
	var length int
	for _, field := range fields {
		var arguments []string
		long, ok := field.Tag.Lookup("long")
		if ok {
			arguments = append(arguments, fmt.Sprintf("--%s", long))
		}

		short, ok := field.Tag.Lookup("short")
		if ok {
			arguments = append(arguments, fmt.Sprintf("-%s", short))
		}

		envs, ok := field.Tag.Lookup("env")
		if ok {
			arguments = append(arguments, strings.Split(envs, ",")...)
		}

		field := strings.Join(arguments, ", ")

		if len(field) > length {
			length = len(field)
		}

		usage = append(usage, field)
	}

	for i, line := range usage {
		usage[i] = pad(line, " ", length)
	}

	for i, field := range fields {
		var kindParts []string
		if _, ok := field.Tag.Lookup("required"); ok {
			kindParts = append(kindParts, "required")
		}

		kind := field.Type.Kind().String()
		if kind == reflect.Slice.String() {
			kind = field.Type.Elem().Kind().String()
			kindParts = append(kindParts, "variadic")
		}

		if len(kindParts) > 0 {
			kind = fmt.Sprintf("%s (%s)", kind, strings.Join(kindParts, ", "))
		}

		line := fmt.Sprintf("%s  %s", usage[i], kind)

		if len(line) > length {
			length = len(line)
		}

		usage[i] = line
	}

	for i, line := range usage {
		usage[i] = pad(line, " ", length)
	}

	for i, field := range fields {
		description, ok := field.Tag.Lookup("description")
		if ok {
			if _, ok := field.Tag.Lookup("deprecated"); ok {
				description = fmt.Sprintf("**DEPRECATED** %s", description)
			}

			if _, ok := field.Tag.Lookup("experimental"); ok {
				description = fmt.Sprintf("**EXPERIMENTAL** %s", description)
			}

			usage[i] += fmt.Sprintf("  %s", description)
		}
	}

	for i, field := range fields {
		defaultValue, ok := field.Tag.Lookup("default")
		if ok {
			usage[i] += fmt.Sprintf(" (default: %s)", defaultValue)
		}
	}

	for i, field := range fields {
		defaultValue, ok := field.Tag.Lookup("default-path")
		if ok {
			usage[i] += fmt.Sprintf(" (default-path: %s)", strings.Join(strings.Split(defaultValue, ","), ", "))
		}
	}

	for i, field := range fields {
		aliases, ok := field.Tag.Lookup("alias")
		if ok {
			var arguments []string
			for _, alias := range strings.Split(aliases, ",") {
				arguments = append(arguments, fmt.Sprintf("--%s", alias))
			}
			usage[i] += fmt.Sprintf("\n  (aliases: %s)", strings.Join(arguments, ", "))
		}
	}

	sort.Strings(usage)

	return strings.Join(usage, "\n"), nil
}

func getFields(t reflect.Type) []reflect.StructField {
	var fields []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Type.Kind() == reflect.Struct {
			fields = append(fields, getFields(field.Type)...)
			continue
		}

		fields = append(fields, field)
	}
	return fields
}

func pad(str, pad string, length int) string {
	for {
		str += pad
		if len(str) > length {
			return str[0:length]
		}
	}
}
