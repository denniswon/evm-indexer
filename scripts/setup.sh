#!/bin/bash

unameOut="$(uname -s)"
case "${unameOut}" in
    Linux*)     machine=Linux;;
    Darwin*)    machine=Mac;;
    CYGWIN*)    machine=Cygwin;;
    MINGW*)     machine=MinGw;;
    *)          machine="UNKNOWN:${unameOut}"
esac

echo "Setting up on ${machine}"

if [ "$machine" == "Mac" ]; then
    # Install dependencies
    which -s brew
    if [[ $? != 0 ]] ; then
        # Install Homebrew
        echo "Installing Homebrew"
        ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
    else
        echo "Homebrew already installed. Running brew update"
        brew update
    fi

    # Install Postgres
    brew tap homebrew/core
    brew list postgresql || brew install postgresql

    brew services restart postgresql

    # Create a user 'postgres'. psql -U postgres to login to postgres locally
    createuser -s postgres
    # Create db. indexer is the db name in .env
    createdb -U postgres -w indexer
    # enable pgcrypto extension
    psql -d indexer -U postgres -c "CREATE EXTENSION pgcrypto;"

    # Optional: create a password for postgres with MD5 authentication
    # First create password for user `psql -d indexer -U postgres`
    # then set password by \password command
    # Finally, on `/opt/homebrew/var/postgresql@14/pg_hba.conf`, change each ident/trust to md5
    # Then, restart postgres with `brew services restart postgresql`

    # Install Redis
    # Install Postgres
    brew list redis || brew install redis
    brew services restart redis

    # Optional: update redis configuration file to allow connections from anywhere.
    # This is not as secure as binding to localhost.
    # From `/opt/homebrew/etc/redis.conf`, uncomment line `bind 127.0.0.1 ::1`
    # Then, restart redis with `brew services restart redis`

    # Optional: configure a redis password
    # From `/opt/homebrew/etc/redis.conf`, uncomment line `# requirepass foobared

fi


