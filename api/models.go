package api

type CVE struct {
	CVEID              string   `json:"cve_id"`
	Summary            *string  `json:"summary"`
	CVSS               *float64 `json:"cvss"`
	CVSSVersion        *float64 `json:"cvss_version"`
	CVSSv2             *float64 `json:"cvss_v2"`
	CVSSv3             *float64 `json:"cvss_v3"`
	EPSS               *float64 `json:"epss"`
	RankingEPSS        *float64 `json:"ranking_epss"`
	KEV                bool     `json:"kev"`
	ProposeAction      *string  `json:"propose_action"`
	RansomwareCampaign *string  `json:"ransomware_campaign"`
	References         []string `json:"references"`
	PublishedTime      string   `json:"published_time"`
	Vendor             *string  `json:"vendor"`
	Product            *string  `json:"product"`
	Version            *string  `json:"version"`
}

type CVEWithCPEs struct {
	CVE
	CPEs []string `json:"cpes"`
}

type CVEs struct {
	CVEs []CVE `json:"cves"`
}

type CPEs struct {
	CPEs []string `json:"cpes"`
}
