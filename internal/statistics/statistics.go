package statistics

type Class struct {
	DisplayName string
	Total       int
}

type Month struct {
	Total  int
	Number int
}

type YearStatistics struct {
	Total   int
	Months  []Month
	Classes []Class
}

type MonthStatistics struct {
	Total   int
	Weeks   []Week
	Classes []Class
}

type Week struct {
	Total int
	// Number is a week number
	Number int
}

type Day struct {
	Total  int
	Number int
}

type WeekStatistics struct {
	Total   int
	Days    []Day
	Classes []Class
}
