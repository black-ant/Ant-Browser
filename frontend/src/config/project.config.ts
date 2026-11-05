import { featuresConfig } from './features.config'
import { navigationConfig } from './navigation.config'
import { projectConfig } from './projectBase.config'
import { uiConfig } from './ui.config'

export { featuresConfig } from './features.config'
export { navigationConfig } from './navigation.config'
export { projectConfig } from './projectBase.config'
export { uiConfig } from './ui.config'
export type { NavItem, NavSection } from './navigation.config'

export default {
  project: projectConfig,
  navigation: navigationConfig,
  features: featuresConfig,
  ui: uiConfig,
}
