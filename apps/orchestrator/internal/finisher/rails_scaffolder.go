// RailsScaffolder — Ruby on Rails 7.2 API-only baseline. Triggers when
// the planner picks Rails or Ruby, or when user stories mention them.
// Pairs with the shared Postgres provisioner — production reads the
// DATABASE_URL the platform injects.
//
// The scaffold is intentionally lean: enough files for `bundle install
// && bundle exec rails s` to boot a /health endpoint and the example
// users resource, then the Coder fills in the real domain. We do NOT
// emit Gemfile.lock; let bundler resolve fresh on first install.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type RailsScaffolder struct{}

func (RailsScaffolder) Name() string { return "ruby-rails" }

func (RailsScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "rails") || strings.Contains(stack, "ruby") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "rails") || strings.Contains(body, "ruby") {
			return true
		}
	}
	return false
}

func (RailsScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"Gemfile": `# Gemfile — Rails 7.2 API service. Pinned to a single major so
# bundler resolves a stable lockfile on the first install. Add new
# gems alphabetically inside the same group.
source 'https://rubygems.org'

ruby '3.3.0'

gem 'rails', '~> 7.2.0'
gem 'pg', '~> 1.5'
gem 'puma', '~> 6.4'
gem 'bootsnap', '~> 1.18', require: false
gem 'rack-cors', '~> 2.0'
gem 'dotenv-rails', '~> 3.1', groups: [:development, :test]
`,
		"config/application.rb": `# Rails application bootstrap. API-only mode strips session +
# cookie middleware we do not need; CORS is opened up at the rack
# layer because the frontend is served from a separate origin.
require_relative 'boot'

require 'rails'
require 'active_model/railtie'
require 'active_job/railtie'
require 'active_record/railtie'
require 'action_controller/railtie'
require 'action_view/railtie'

Bundler.require(*Rails.groups)

module IronflyerApp
  class Application < Rails::Application
    config.load_defaults 7.2
    config.api_only = true
    config.autoload_lib(ignore: %w[assets tasks])

    config.middleware.insert_before 0, Rack::Cors do
      allow do
        origins '*'
        resource '*', headers: :any, methods: [:get, :post, :put, :patch, :delete, :options, :head]
      end
    end
  end
end
`,
		"config/boot.rb": `# Standard Rails boot file. Keep bootsnap on for faster cold
# starts in the container runtime.
ENV['BUNDLE_GEMFILE'] ||= File.expand_path('../Gemfile', __dir__)
require 'bundler/setup'
require 'bootsnap/setup'
`,
		"config/environment.rb": `# Loads the Rails application then runs initializers.
require_relative 'application'
Rails.application.initialize!
`,
		"config/routes.rb": `# HTTP route table. Keep the example resources entry until the
# Coder swaps it for the real domain model. /health is the
# readiness probe the platform pings.
Rails.application.routes.draw do
  get '/health', to: 'health#show'
  resources :users
end
`,
		"config/database.yml": `# Database config. Production reads DATABASE_URL exactly as the
# platform injects it; dev/test fall back to a local postgres so
# the scaffold runs without further setup.
default: &default
  adapter: postgresql
  encoding: unicode
  pool: <%= ENV.fetch('RAILS_MAX_THREADS', 5) %>

development:
  <<: *default
  database: app_development
  host: <%= ENV.fetch('DB_HOST', 'localhost') %>
  username: <%= ENV.fetch('DB_USER', 'postgres') %>
  password: <%= ENV.fetch('DB_PASSWORD', '') %>

test:
  <<: *default
  database: app_test
  host: <%= ENV.fetch('DB_HOST', 'localhost') %>

production:
  <<: *default
  url: <%= ENV['DATABASE_URL'] %>
`,
		"config/puma.rb": `# Puma config. Threads scale with RAILS_MAX_THREADS; worker count
# uses WEB_CONCURRENCY so the container can be sized from outside.
max_threads_count = ENV.fetch('RAILS_MAX_THREADS', 5).to_i
threads max_threads_count, max_threads_count

port        ENV.fetch('PORT', 3000)
environment ENV.fetch('RAILS_ENV', 'production')
workers     ENV.fetch('WEB_CONCURRENCY', 2).to_i
preload_app!
plugin :tmp_restart
`,
		"config.ru": `# Rack entry point used by puma + rails s.
require_relative 'config/environment'
run Rails.application
Rails.application.load_server
`,
		"app/controllers/application_controller.rb": `# Base controller for the API. Every other controller inherits
# from here so cross-cutting concerns (auth filters, error
# rendering) land in one place.
class ApplicationController < ActionController::API
end
`,
		"app/controllers/users_controller.rb": `# Example CRUD controller. Swap the strong-params list and any
# additional filtering once the User model grows beyond the
# scaffolded :email column.
class UsersController < ApplicationController
  before_action :set_user, only: [:show, :update, :destroy]

  def index
    render json: User.all
  end

  def show
    render json: @user
  end

  def create
    user = User.new(user_params)
    if user.save
      render json: user, status: :created
    else
      render json: { errors: user.errors }, status: :unprocessable_entity
    end
  end

  def update
    if @user.update(user_params)
      render json: @user
    else
      render json: { errors: @user.errors }, status: :unprocessable_entity
    end
  end

  def destroy
    @user.destroy
    head :no_content
  end

  private

  def set_user
    @user = User.find(params[:id])
  end

  def user_params
    params.require(:user).permit(:email)
  end
end
`,
		"app/controllers/health_controller.rb": `# Liveness + readiness endpoint. Always returns 200 with a tiny
# JSON body so the platform can distinguish "process up" from
# "no response".
class HealthController < ApplicationController
  def show
    render json: { status: 'ok' }
  end
end
`,
		"app/models/user.rb": `# Example ActiveRecord model. The matching migration is at
# db/migrate/20260524000000_create_users.rb. Add validations,
# associations, and scopes here as the domain grows.
class User < ApplicationRecord
end
`,
		"db/migrate/20260524000000_create_users.rb": `# Example migration. Run with: bundle exec rails db:migrate
class CreateUsers < ActiveRecord::Migration[7.2]
  def change
    create_table :users do |t|
      t.string :email, null: false
      t.timestamps
    end
    add_index :users, :email, unique: true
  end
end
`,
		"db/seeds.rb": `# Seed data is run via: bundle exec rails db:seed
# Keep this idempotent so reruns do not duplicate rows.
`,
		"bin/rails": `#!/usr/bin/env bash
# Thin wrapper so contributors can run ./bin/rails ... without
# remembering to prefix bundle exec. CI also calls this script.
set -euo pipefail
exec bundle exec rails "$@"
`,
		"Dockerfile": `# Multi-stage Rails image. Build stage installs gems into a
# layered cache; runtime stage copies the resolved bundle and the
# app source, then boots puma.
FROM ruby:3.3-slim AS build
WORKDIR /app
ENV BUNDLE_DEPLOYMENT=1 BUNDLE_PATH=/usr/local/bundle BUNDLE_WITHOUT=development:test
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends build-essential libpq-dev git && \
    rm -rf /var/lib/apt/lists/*
COPY Gemfile ./
RUN bundle install
COPY . .

FROM ruby:3.3-slim AS runtime
WORKDIR /app
ENV RAILS_ENV=production RAILS_LOG_TO_STDOUT=1 RAILS_SERVE_STATIC_FILES=1
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends libpq5 tzdata && \
    rm -rf /var/lib/apt/lists/*
COPY --from=build /usr/local/bundle /usr/local/bundle
COPY --from=build /app /app
EXPOSE 3000
CMD ["bundle", "exec", "puma", "-C", "config/puma.rb"]
`,
		".dockerignore": `.git
.gitignore
log/*
tmp/*
node_modules
.bundle
vendor/bundle
storage
*.log
.env
.env.*
`,
		".gitignore": `/.bundle
/log/*
/tmp/*
!/log/.keep
!/tmp/.keep
/storage/*
!/storage/.keep
/public/assets
.env
.env.*
!.env.example
/coverage/
/spec/examples.txt
/node_modules
`,
	}
	contract := `Rails scaffold: Ruby on Rails 7.2 API-only, PostgreSQL, puma.

Already provisioned:
- /Gemfile                                       → rails 7.2 + pg + puma + rack-cors + dotenv-rails
- /config/application.rb                         → API-only app, CORS open
- /config/routes.rb                              → resources :users + GET /health
- /config/database.yml                           → production reads DATABASE_URL
- /config/puma.rb, /config/boot.rb, /config.ru   → server boot
- /app/controllers/application_controller.rb     → ActionController::API base
- /app/controllers/users_controller.rb           → example CRUD
- /app/controllers/health_controller.rb          → GET /health
- /app/models/user.rb                            → ActiveRecord stub
- /db/migrate/20260524000000_create_users.rb     → users table migration
- /bin/rails                                     → bundle exec rails wrapper
- /Dockerfile                                    → multi-stage ruby:3.3-slim

Contract for the Coder:
1. Install deps + boot: bundle install && bundle exec rails s
2. Database: DATABASE_URL must be set in production. Migrate with
   bundle exec rails db:migrate (Gemfile.lock is intentionally absent —
   let bundler resolve on first install).
3. Add new resources by generating a model + controller + migration,
   then register the route in /config/routes.rb.
4. Keep API-only mode on unless server-rendered views are explicitly
   requested. If you switch, remove config.api_only and re-add the
   middleware Rails strips.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
