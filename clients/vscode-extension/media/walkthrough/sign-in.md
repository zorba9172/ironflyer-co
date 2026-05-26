# Sign in

`Ironflyer: Sign In` opens `localhost:3000/login` (or whatever `ironflyer.webUrl` points to) with a callback URL pointing back to this extension. After you complete the login on the web — email/password, Google, or GitHub — the web app redirects to `vscode://ironflyer.ironflyer/auth?token=…` and the extension picks up the JWT.

The token is stored in VSCode's `SecretStorage`. It never lands in `settings.json` and is not exposed to other extensions.

If you sign in from a fresh machine, point `ironflyer.orchestratorUrl` and `ironflyer.webUrl` at your deployment before clicking Sign In.
