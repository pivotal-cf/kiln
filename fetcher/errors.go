package fetcher

type stringError string

func (str stringError) Error() string { return string(str) }

type multipleErrors []error

func (errs multipleErrors) Error() string { return strconv.Itoa(len(errs)) + " errors" }
