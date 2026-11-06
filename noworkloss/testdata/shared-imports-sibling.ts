import { adoptShadowStyles } from './shadow-styles.ts';
import { onFirstListener } from './listener-watch.ts';
import { upgradeProperties } from './upgrade-properties.ts';
import { categoryColor, categoryHue } from './color.ts';
export * from './table-math.ts';

export class TableElement extends HTMLElement {
  connectedCallback(): void {
    upgradeProperties(this, TableElement.prototype);
    this.renderTheWholeTable();
  }
}
