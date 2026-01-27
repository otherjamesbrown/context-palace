# Context-Palace Authentication Setup

SSL client certificate authentication for PostgreSQL. No passwords needed once set up.

## Overview

```
┌─────────────────────────────────────────┐
│  PostgreSQL (dev02.brown.chat)          │
│                                         │
│  Validates client certificates against  │
│  trusted CA certificate                 │
└────────────────┬────────────────────────┘
                 │
        SSL/TLS with client cert
                 │
    ┌────────────┼────────────┐
    │            │            │
┌───┴───┐   ┌────┴───┐   ┌────┴───┐
│ dev01 │   │ dev02  │   │ laptop │
│       │   │        │   │        │
│ cert  │   │ cert   │   │ cert   │
└───────┘   └────────┘   └────────┘
```

## Part 1: Create Certificate Authority (One-time)

Run on a secure machine (e.g., dev02 or your laptop):

```bash
# Create directory for CA
mkdir -p ~/context-palace-ca
cd ~/context-palace-ca

# Generate CA private key
openssl genrsa -out ca.key 4096

# Generate CA certificate (valid 10 years)
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
  -subj "/CN=Context-Palace CA/O=brown.chat"

# Secure the CA key
chmod 600 ca.key
```

**Output:**
- `ca.key` - CA private key (keep secure, needed to sign new certs)
- `ca.crt` - CA certificate (distribute to server and clients)

## Part 2: Configure PostgreSQL Server

### 2.1 Copy CA cert to PostgreSQL

```bash
# On dev02
sudo cp ~/context-palace-ca/ca.crt /var/lib/postgresql/data/ca.crt
sudo chown postgres:postgres /var/lib/postgresql/data/ca.crt
```

### 2.2 Enable SSL in postgresql.conf

```bash
# Edit postgresql.conf
sudo nano /var/lib/postgresql/data/postgresql.conf
```

Add/modify:
```
ssl = on
ssl_cert_file = '/var/lib/postgresql/data/server.crt'
ssl_key_file = '/var/lib/postgresql/data/server.key'
ssl_ca_file = '/var/lib/postgresql/data/ca.crt'
```

### 2.3 Generate server certificate (if not exists)

```bash
cd ~/context-palace-ca

# Generate server key
openssl genrsa -out server.key 2048

# Generate server CSR
openssl req -new -key server.key -out server.csr \
  -subj "/CN=dev02.brown.chat"

# Sign with CA
openssl x509 -req -days 3650 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt

# Install
sudo cp server.crt server.key /var/lib/postgresql/data/
sudo chown postgres:postgres /var/lib/postgresql/data/server.*
sudo chmod 600 /var/lib/postgresql/data/server.key
```

### 2.4 Configure pg_hba.conf for cert auth

```bash
sudo nano /var/lib/postgresql/data/pg_hba.conf
```

Add line for certificate authentication:
```
# TYPE  DATABASE        USER            ADDRESS                 METHOD
hostssl contextpalace   all             0.0.0.0/0               cert clientcert=verify-full
```

This means:
- `hostssl` - SSL connections only
- `contextpalace` - Only this database
- `all` - Any user (username comes from cert CN)
- `0.0.0.0/0` - Any IP
- `cert clientcert=verify-full` - Require valid client certificate

### 2.5 Restart PostgreSQL

```bash
sudo systemctl restart postgresql
```

## Part 3: Create Client Certificates

For each agent/machine that needs access:

```bash
cd ~/context-palace-ca

# Set the agent/machine name
AGENT_NAME="agent-dev01"  # or "agent-laptop", "human-james", etc.

# Generate client key
openssl genrsa -out ${AGENT_NAME}.key 2048

# Generate client CSR (CN becomes the PostgreSQL username)
openssl req -new -key ${AGENT_NAME}.key -out ${AGENT_NAME}.csr \
  -subj "/CN=${AGENT_NAME}"

# Sign with CA
openssl x509 -req -days 365 -in ${AGENT_NAME}.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out ${AGENT_NAME}.crt

# Package for distribution
tar czf ${AGENT_NAME}-certs.tar.gz ${AGENT_NAME}.crt ${AGENT_NAME}.key ca.crt
```

**Output:** `agent-dev01-certs.tar.gz` containing:
- `agent-dev01.crt` - Client certificate
- `agent-dev01.key` - Client private key
- `ca.crt` - CA certificate (to verify server)

## Part 4: Install Client Certificates

On each client machine:

```bash
# Create PostgreSQL cert directory
mkdir -p ~/.postgresql
chmod 700 ~/.postgresql

# Extract certs
tar xzf agent-dev01-certs.tar.gz -C ~/.postgresql

# Rename to standard names
mv ~/.postgresql/agent-dev01.crt ~/.postgresql/postgresql.crt
mv ~/.postgresql/agent-dev01.key ~/.postgresql/postgresql.key
mv ~/.postgresql/ca.crt ~/.postgresql/root.crt

# Secure permissions
chmod 600 ~/.postgresql/postgresql.key
chmod 644 ~/.postgresql/postgresql.crt ~/.postgresql/root.crt
```

## Part 5: Create Database User for Certificate

On PostgreSQL server:

```bash
# Create user matching the certificate CN
sudo -u postgres psql -d contextpalace -c "CREATE USER \"agent-dev01\";"
sudo -u postgres psql -d contextpalace -c "GRANT ALL ON ALL TABLES IN SCHEMA public TO \"agent-dev01\";"
sudo -u postgres psql -d contextpalace -c "GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO \"agent-dev01\";"
```

## Part 6: Connect

Once set up, connection is automatic:

```bash
# psql finds certs in ~/.postgresql automatically
psql "host=dev02.brown.chat dbname=contextpalace sslmode=verify-full"

# Or explicit paths
psql "host=dev02.brown.chat dbname=contextpalace sslmode=verify-full \
  sslcert=~/.postgresql/postgresql.crt \
  sslkey=~/.postgresql/postgresql.key \
  sslrootcert=~/.postgresql/root.crt"
```

No password needed. Certificate proves identity.

## Connection String for Agents

Update `.env` or CLAUDE.md:

```bash
# No password!
CONTEXT_PALACE_DATABASE_URL="postgresql://dev02.brown.chat:5432/contextpalace?sslmode=verify-full"

# psql command
psql "host=dev02.brown.chat dbname=contextpalace sslmode=verify-full"
```

## Managing Certificates

### Add a new agent/machine

```bash
cd ~/context-palace-ca
AGENT_NAME="agent-newmachine"

openssl genrsa -out ${AGENT_NAME}.key 2048
openssl req -new -key ${AGENT_NAME}.key -out ${AGENT_NAME}.csr -subj "/CN=${AGENT_NAME}"
openssl x509 -req -days 365 -in ${AGENT_NAME}.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out ${AGENT_NAME}.crt

# Create DB user
psql "host=dev02.brown.chat dbname=contextpalace sslmode=verify-full" -c "CREATE USER \"${AGENT_NAME}\";"
psql "host=dev02.brown.chat dbname=contextpalace sslmode=verify-full" -c "GRANT ALL ON ALL TABLES IN SCHEMA public TO \"${AGENT_NAME}\";"
```

### Revoke an agent's access

```bash
# Just drop the user - cert becomes useless
psql "host=dev02.brown.chat dbname=contextpalace sslmode=verify-full" \
  -c "DROP USER \"agent-compromised\";"
```

Or for immediate revocation, add cert serial to a CRL (Certificate Revocation List).

### Renew expiring certificates

```bash
# Same process as creating - sign new cert with same CN
AGENT_NAME="agent-dev01"
openssl req -new -key ${AGENT_NAME}.key -out ${AGENT_NAME}.csr -subj "/CN=${AGENT_NAME}"
openssl x509 -req -days 365 -in ${AGENT_NAME}.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out ${AGENT_NAME}.crt
```

## Security Notes

1. **Protect ca.key** - Anyone with this can create valid certs
2. **Protect client .key files** - These are like passwords
3. **Use short expiry for client certs** - 1 year, renew annually
4. **Cert CN = DB username** - Access control via PostgreSQL GRANT/REVOKE
5. **One cert per machine** - Easy to revoke if compromised

## Troubleshooting

### "certificate verify failed"
- Check `sslrootcert` points to correct `ca.crt`
- Verify server cert was signed by same CA

### "permission denied for user"
- Ensure DB user exists with matching CN: `CREATE USER "agent-name";`
- Ensure grants: `GRANT ALL ON ALL TABLES IN SCHEMA public TO "agent-name";`

### "private key doesn't match certificate"
- Regenerate both key and cert together

### Check cert details
```bash
openssl x509 -in ~/.postgresql/postgresql.crt -text -noout
```

## Quick Reference

| File | Location | Purpose |
|------|----------|---------|
| `ca.key` | Secure storage | Signs new certs (keep safe!) |
| `ca.crt` | Server + all clients | Verifies certs |
| `server.crt` | PostgreSQL server | Server identity |
| `server.key` | PostgreSQL server | Server private key |
| `postgresql.crt` | `~/.postgresql/` on client | Client identity |
| `postgresql.key` | `~/.postgresql/` on client | Client private key |
| `root.crt` | `~/.postgresql/` on client | Verifies server |
