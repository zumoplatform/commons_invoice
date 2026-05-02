module github.com/zumoplatform/commons_invoice

go 1.26.2

require (
	github.com/zumoplatform/commons_customer v0.0.0-00010101000000-000000000000
	github.com/zumoplatform/commons_invoice_item v0.0.0-00010101000000-000000000000
	github.com/zumoplatform/commons_organization v0.0.0-00010101000000-000000000000
	gorm.io/gorm v1.31.1
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/zumoplatform/commons_product v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/text v0.20.0 // indirect
)

replace (
	github.com/zumoplatform/commons_customer => ../commons_customer
	github.com/zumoplatform/commons_invoice_item => ../commons_invoice_item
	github.com/zumoplatform/commons_organization => ../commons_organization
	github.com/zumoplatform/commons_product => ../commons_product
)
