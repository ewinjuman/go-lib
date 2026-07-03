package database

type Option struct {
	// DbType selects the driver: "mysql" or "postgres". Empty defaults to postgres.
	DbType string `json:"dbType"`
	// Username is the database login user.
	Username string `json:"username"`
	// Password is the database login password.
	Password string `json:"password"`
	// Schema is the database/schema name to connect to.
	Schema string `json:"schema"`
	// Host is the database server host.
	Host string `json:"host"`
	// Port is the database server port.
	Port int `json:"port"`
	// MaxIdleConn sets the maximum number of idle connections kept in the pool.
	MaxIdleConn int `json:"maxIdleConn"`
	// MaxOpenConn sets the maximum number of open connections to the database.
	MaxOpenConn int `json:"maxOpenConn"`
	// LogMode enables GORM query logging (routed through the *logger.Logger
	// passed to NewConnection) when true; silent otherwise.
	LogMode bool `json:"logMode"`
	// SslMode enables TLS for the connection (Postgres only; ignored for MySQL).
	SslMode bool `json:"sslMode"`
	// Timezone sets the connection timezone. Empty defaults to "Asia/Jakarta".
	Timezone string `json:"timezone"`
}
