package fetcher

type stringError string

func (str stringError) Error() string { return string(str) }
