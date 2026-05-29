import { injectable } from '@theia/core/shared/inversify';
import { FrontendApplicationContribution } from '@theia/core/lib/browser';

const PRODUCT_NAME = 'Ironflyer IDE';

/**
 * Applies Ironflyer branding to the Theia browser shell:
 *  - forces the document/window title to the product name,
 *  - removes the default Theia "Getting Started" tab if one opened,
 *  - strips Theia attribution from the About dialog where reachable,
 *  - injects a lightweight Ironflyer wordmark, no Electron menu.
 */
@injectable()
export class IronflyerBrandingContribution implements FrontendApplicationContribution {

    // Runs once the frontend application has finished its initial layout.
    onStart(): void {
        this.applyTitle();
        this.observeTitle();
        this.installWordmark();
        this.closeGettingStarted();
        this.scrubAboutAttribution();
    }

    private applyTitle(): void {
        document.title = PRODUCT_NAME;
    }

    /**
     * Theia rewrites document.title as the active editor changes
     * (e.g. "file.ts — Ironflyer IDE"). The applicationName in package.json
     * already supplies the product name, so this guard only strips any raw
     * "Theia" string that leaks through, leaving the rest of the title intact.
     */
    private observeTitle(): void {
        const titleEl = document.querySelector('title');
        if (!titleEl) {
            return;
        }
        const fix = (): void => {
            const current = document.title;
            const stripped = current.replace(/theia/gi, PRODUCT_NAME);
            if (stripped !== current) {
                document.title = stripped;
            }
        };
        const observer = new MutationObserver(fix);
        observer.observe(titleEl, { childList: true });
        fix();
    }

    /**
     * Inject the Ironflyer gate mark + "Ironflyer" wordmark into the top-right
     * of the shell. The mark is the official gate/arrow geometry (mirrors
     * clients/studio/src/components/LogoMark.tsx) filled with the cobalt->cyan
     * signature gradient. A per-instance gradient id avoids collisions with any
     * other inline SVG defs the workbench injects. Kept lightweight,
     * pointer-events:none and theme-colored.
     */
    private installWordmark(): void {
        if (document.getElementById('ironflyer-wordmark')) {
            return;
        }
        const gradientId = `ironflyer-mark-gradient-${Date.now().toString(36)}`;
        const mark = document.createElement('div');
        mark.id = 'ironflyer-wordmark';
        mark.setAttribute('aria-label', PRODUCT_NAME);
        mark.innerHTML =
            `<svg class="ironflyer-wordmark-svg" viewBox="0 0 40 40" width="18" height="18" `
            + `aria-hidden="true" focusable="false">`
            + `<defs>`
            + `<linearGradient id="${gradientId}" x1="6" y1="34" x2="34" y2="6" `
            + `gradientUnits="userSpaceOnUse">`
            + `<stop offset="0" stop-color="#2F6BFF"/>`
            + `<stop offset="1" stop-color="#18C8E6"/>`
            + `</linearGradient>`
            + `</defs>`
            + `<path d="M6 34 L20 6 L34 34 L20 26 Z" fill="url(#${gradientId})"/>`
            + `</svg>`
            + `<span class="ironflyer-wordmark-text">Ironflyer</span>`;
        document.body.appendChild(mark);
    }

    /**
     * If a Getting Started widget ever opens (it is excluded from the
     * dependency set, but a plugin could contribute one), close it.
     */
    private closeGettingStarted(): void {
        const close = () => {
            const tabs = document.querySelectorAll('.p-TabBar-tab, .lm-TabBar-tab');
            tabs.forEach(tab => {
                const label = tab.querySelector('.p-TabBar-tabLabel, .lm-TabBar-tabLabel');
                if (label && /getting started|welcome/i.test(label.textContent || '')) {
                    const closeIcon = tab.querySelector('.p-TabBar-tabCloseIcon, .lm-TabBar-tabCloseIcon');
                    (closeIcon as HTMLElement | null)?.click();
                }
            });
        };
        // Retry briefly while the workbench finishes restoring its layout.
        let attempts = 0;
        const timer = setInterval(() => {
            close();
            if (++attempts > 10) {
                clearInterval(timer);
            }
        }, 500);
    }

    /**
     * Best-effort removal of Theia attribution in the About dialog. Theia
     * builds the dialog lazily, so observe the DOM and scrub when it appears.
     */
    private scrubAboutAttribution(): void {
        const scrub = (root: ParentNode) => {
            root.querySelectorAll('.theia-aboutDialog, .dialogContent').forEach(node => {
                node.querySelectorAll('a, p, h1, h2, span').forEach(el => {
                    const text = el.textContent || '';
                    if (/eclipse theia|theia ide|theia\b/i.test(text)) {
                        el.textContent = text.replace(/eclipse theia|theia ide|theia/gi, PRODUCT_NAME);
                    }
                });
            });
        };
        const observer = new MutationObserver(records => {
            for (const r of records) {
                r.addedNodes.forEach(n => {
                    if (n instanceof HTMLElement) {
                        scrub(n);
                    }
                });
            }
        });
        observer.observe(document.body, { childList: true, subtree: true });
    }
}
