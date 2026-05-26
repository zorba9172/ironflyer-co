import type { ExpoConfig } from 'expo/config'

import appJson from './app.json'

const base = (appJson as { expo: ExpoConfig }).expo

const config: ExpoConfig = {
  ...base,
  extra: {
    ...(base.extra ?? {}),
    eas: {
      projectId: process.env.EAS_PROJECT_ID ?? ''
    }
  }
}

export default config
