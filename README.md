# daily-kopi

A daily news digest: pulls curated RSS feeds per topic, skips links already seen, and sends them to Telegram. Built with Go, RSS via gofeed, and Telegram Bot API.

## Running locally

1. Install Go 1.27+.

2. [Create a Telegram bot](https://core.telegram.org/bots/features#botfather) with its corresponding token and place the latter's value in an `.env` file:

   ```
   TELEGRAM_BOT_TOKEN=<your-token-here>
   ```

3. Copy `config.example.toml` into `config.toml`, then add, remove or replace values where necessary. To receive a valid `recipients.chat_id`, message the newly-created bot, then read `chat.id` from `https://api.telegram.org/bot<token>/getUpdates`.

4. Run the application:

   ```bash
   go run .
   ```

## Configuration

| Field                 | Description                                                               | Default value |
| --------------------- | ------------------------------------------------------------------------- | ------------- |
| `state_file`          | Where sent links are recorded                                             | state.json    |
| `max_items_per_topic` | Max number of feed items sent per topic                                   | 5             |
| `topics`              | List of topics keyed by `name` with its `display_name` (optional)         | **required**  |
| `topics.feeds`        | Feeds for a topic, items taken round-robin, list the most important first | **required**  |
| `recipients`          | `chat_id` and the topic `name`s they receive (must match a defined topic) | **required**  |

## Running on GitHub Actions

This application primarily runs on daily scheduled cron jobs, triggered automatically through [cron-job.org](https://cron-job.org/en/) calling `workflow_dispatch` at 07:00 SGT, using a fine-grained PAT scoped to this repo with **Actions: Read and write**.

Sent links are tracked in `state.json` which is carried between runs by `actions/cache`. If the cache is lost, the only effect is some repeated links.

A run fails (goes red) when:

- any send fails, even if other recipients succeeded

- every feed fails to fetch

Retrying via `gh workflow run digest.yml` or through the GitHub UI is possible. There will be no repeated links sent to the prior successful recipients.

Repository secrets:

| Name                 | Description                                                          |
| -------------------- | -------------------------------------------------------------------- |
| `CONFIG_TOML`        | Entire config file, to update whenever a config change is to be made |
| `TELEGRAM_BOT_TOKEN` | Needed for Telegram API, to run app                                  |

Whenever configurations need updating, update the `CONFIG_TOML` repository secret either in the repo settings (Settings -> Secrets and variables -> Actions), or via the GitHub CLI (`gh`):

```bash
gh secret set CONFIG_TOML < path/to/config.toml
```

## Roadmap

See the [backlog issues](https://github.com/erisjacey/daily-kopi/issues?q=is:open+label:backlog).

## License

MIT, see [LICENSE](LICENSE).
