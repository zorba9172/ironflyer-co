import { ContainerModule } from '@theia/core/shared/inversify';
import { FrontendApplicationContribution } from '@theia/core/lib/browser';
import { IronflyerBrandingContribution } from './ironflyer-branding-contribution';

// Pull in brand CSS (UI font + chrome de-clutter) so it is bundled by the
// Theia webpack build alongside the extension.
import '../../style/branding.css';

// Theia auto-loads the default export of the module referenced in
// package.json -> theiaExtensions[].frontend.
export default new ContainerModule(bind => {
    bind(IronflyerBrandingContribution).toSelf().inSingletonScope();
    bind(FrontendApplicationContribution).toService(IronflyerBrandingContribution);
});
