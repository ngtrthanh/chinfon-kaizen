package app

import (
	"database/sql"
	"log"
)

// Config controls the single-binary application.
type Config struct {
	DBPath       string
	UploadDir    string
	CookieSecure bool
	SeedDemo     bool
	Logger       *log.Logger
}

type App struct {
	DB              *sql.DB
	Templates       *templateSet
	UploadDir       string
	CookieSecure    bool
	SeedDemo        bool
	Logger          *log.Logger
	loginLimiter    *loginLimiter
	SessionCookie   string
	LoginCSRFCookie string
	MFAKey          []byte
	MFAKeyPath      string
}

type User struct {
	ID                 int64
	Username           string
	EmployeeNo         string
	FullName           string
	DepartmentCode     string
	DepartmentName     string
	Scope              string
	Roles              []string
	Permissions        map[string]bool
	CSRFToken          string
	MustChangePassword bool
	DemoMode           bool
	MFAPassed          bool
	MFAEnrolled        bool
}

type Option struct {
	Code string
	Name string
}

type Department struct {
	Code      string
	Name      string
	Kind      string
	SortOrder int
	Active    bool
}

type Employee struct {
	EmployeeNo     string
	FullName       string
	DepartmentCode string
	DepartmentName string
	PositionTitle  string
	WorkGroup      string
	Active         bool
}

type Initiative struct {
	ID                       int64
	Code                     string
	TrackCode                string
	TrackName                string
	StageCode                string
	StageName                string
	RecordStatus             string
	Title                    string
	DepartmentCode           string
	DepartmentName           string
	SubmitterEmployeeNo      string
	SubmitterName            string
	EquipmentCode            string
	ProblemSummary           string
	ProblemDetail            string
	SolutionSummary          string
	SolutionDetail           string
	BenefitSummary           string
	ImplementationStatusCode string
	ImplementationStatusName string
	GrossValueVND            int64
	InvestCostVND            int64
	NetBenefitVND            int64
	Annex03                  bool
	RewardBasis              string
	NonFinancialLevel        int
	RewardTierCode           string
	RewardTierName           string
	BaseRewardVND            int64
	RewardAmountVND          int64
	SeedGranted              bool
	Anonymous                bool
	SplitRatio               string
	DueAt                    sql.NullString
	SubmittedAt              sql.NullString
	PaidAt                   sql.NullString
	PaidBy                   sql.NullString
	PaidByName               sql.NullString
	RejectReason             sql.NullString
	CreatedBy                string
	CreatedAt                string
	UpdatedAt                string
	ReviewCount              int
	AttachmentCount          int
	IsDemo                   bool
}

type Participant struct {
	EmployeeNo     string
	FullName       string
	DepartmentName string
	SharePercent   float64
	RoleNote       string
}

type Attachment struct {
	ID               int64
	DocumentTypeCode string
	DocumentTypeName string
	OriginalFilename string
	StorageKey       string
	MimeType         string
	SizeBytes        int64
	SHA256Hex        string
	UploadedBy       string
	UploadedByName   string
	UploadedAt       string
}

type Review struct {
	ID                int64
	StageCode         string
	StageName         string
	ReviewerName      string
	Decision          string
	Comment           string
	AssessedGrossVND  sql.NullInt64
	AssessedInvestVND sql.NullInt64
	AssessedNetVND    sql.NullInt64
	RewardTierCode    sql.NullString
	RewardTierName    sql.NullString
	RewardAmountVND   sql.NullInt64
	CreatedAt         string
}

type HistoryItem struct {
	ID          int64
	FromStage   string
	ToStage     string
	FromName    string
	ToName      string
	ActionCode  string
	ChangedBy   string
	ChangedName string
	Note        string
	CreatedAt   string
}

type RewardTier struct {
	Code          string
	Name          string
	Basis         string
	MinNetVND     sql.NullInt64
	MaxNetVND     sql.NullInt64
	BaseAmountVND int64
	RuleNote      string
	SortOrder     int
}

type RewardRow struct {
	InitiativeID   int64
	Code           string
	TrackCode      string
	DepartmentName string
	EmployeeNo     string
	EmployeeName   string
	SharePercent   float64
	AmountVND      int64
	TierName       string
	PaidAt         sql.NullString
	StageCode      string
}

type RewardTotals struct {
	ApprovedVND int64
	PaidVND     int64
	PendingVND  int64
	Headcount   int
}

type AreaStat struct {
	Code   string
	Name   string
	NetVND int64
	Count  int
	Pct    int
}

type DashboardStats struct {
	Total          int
	Open           int
	NetVND         int64
	RewardApproved int64
	RewardPaid     int64
	Confidential   int
	Fast48         int
	Overdue        int
	Ideas          int
	Applied        int
	Stages         map[string]int
	Tracks         map[string]int
	Areas          []AreaStat
	TopUnits       []AreaStat
	Recent         []Initiative
	OverdueList    []Initiative
}

type UserRow struct {
	ID          int64
	Username    string
	EmployeeNo  string
	FullName    string
	Scope       string
	ScopeName   string
	Roles       []string
	RoleNames   string
	Active      bool
	CreatedAt   string
	MFAEnrolled bool
}

type Role struct {
	Code        string
	Name        string
	Description string
}

type Permission struct {
	Code        string
	Name        string
	Description string
}

type AuditEntry struct {
	ID     int64
	At     string
	Actor  string
	Action string
	Target string
	Status string
	Detail string
	IP     string
}

type PageData struct {
	User       *User
	Title      string
	Subtitle   string
	Active     string
	Error      string
	Message    string
	LoginCSRF  string
	HTTPStatus int

	Initiatives  []Initiative
	Initiative   *Initiative
	Participants []Participant
	Reviews      []Review
	History      []HistoryItem
	Attachments  []Attachment

	Stats *DashboardStats

	RewardTiers  []RewardTier
	NonFinancial []RewardTier
	RewardRows   []RewardRow
	RewardTotals RewardTotals
	OPL          []Initiative

	Departments            []Option
	DepartmentMaster       []Department
	Employees              []Employee
	Users                  []UserRow
	Roles                  []Role
	Permissions            []Permission
	Matrix                 map[string]map[string]bool
	Audit                  []AuditEntry
	DocumentTypes          []Option
	ImplementationStatuses []Option
	Tracks                 []Option
	Scopes                 []Option
	Stages                 []Option

	Filters map[string]string
	Form    map[string]string
	Tab     string

	CanEdit       bool
	CanSubmit     bool
	CanAct        bool
	CanContribute bool
	ActPerm       string
	ActLabel      string
	DefaultReward RewardResult
	SelfReview    bool

	MFASecret    string
	MFAURI       string
	MFARemaining int

	Staff                []StaffRow
	StaffCount           int
	StaffAccountCount    int
	StaffMustChangeCount int
	HasCouncil           bool
	ImportResult         *StaffImportResult
}

type StaffRow struct {
	EmployeeNo     string
	FullName       string
	JobTitle       string
	DepartmentCode string
	DepartmentName string
	ReportTo       string
	Email          string
	MobileNum      string
	Group          string
	Active         bool
	Username       string
	Scope          string
	Roles          []string
	MustChange     bool
	HasAccount     bool
	MFAEnrolled    bool
}

type ImportRowError struct {
	Row    int
	EmpID  string
	Reason string
}

type StaffImportResult struct {
	Total             int
	Departments       int
	EmployeesUpserted int
	UsersCreated      int
	UsersUpdated      int
	AccountsSkipped   int
	SkippedRows       int
	DemoDisabled      bool
	Errors            []ImportRowError
	Notice            string
}

func (pd PageData) CSRF() string {
	if pd.User == nil {
		return ""
	}
	return pd.User.CSRFToken
}
