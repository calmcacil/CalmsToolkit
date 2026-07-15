# Requests examples

Start the menu with `calmstoolkit requests`. Use the search action, enter a movie or series title, choose a match, and confirm the request. TV results allow all or selected seasons. Administrators can open pending requests to approve, decline, and choose service/root-folder overrides.

For a temporary server override:

```bash
CALMSTOOLKIT_REQUESTS_API_KEY="$TOKEN" \
  calmstoolkit requests --url http://requests.example:5055
```

The command is intentionally interactive and has no machine-output mode. Scripts should use the service API directly until a non-interactive requests operation is added.
