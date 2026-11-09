/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import type { GlobalThemeOverrides } from 'naive-ui';

// Values mirror src/styles/tokens.scss (docs/design/console/Foundations).
// CConfigProvider replaces the kit default theme with whatever is passed here,
// so these objects are complete.

const FONT_FAMILY = "Inter, system-ui, -apple-system, 'Segoe UI', sans-serif";
const FONT_FAMILY_MONO =
  "'JetBrains Mono', ui-monospace, 'SFMono-Regular', Menlo, monospace";

function build(palette: {
  primary: string;
  primaryHover: string;
  primaryPressed: string;
  solid: string;
  solidHover: string;
  info: string;
  success: string;
  warning: string;
  error: string;
  text1: string;
  text2: string;
  text3: string;
  disabled: string;
  body: string;
  card: string;
  popover: string;
  subtle: string;
  border: string;
  borderSubtle: string;
  hover: string;
}): GlobalThemeOverrides {
  return {
    common: {
      fontFamily: FONT_FAMILY,
      fontFamilyMono: FONT_FAMILY_MONO,
      fontWeight: '400',
      fontWeightStrong: '600',
      borderRadius: '8px',
      borderRadiusSmall: '6px',
      baseColor: '#FFFFFF',
      primaryColor: palette.primary,
      primaryColorHover: palette.primaryHover,
      primaryColorPressed: palette.primaryPressed,
      primaryColorSuppl: palette.primaryHover,
      infoColor: palette.info,
      infoColorHover: palette.info,
      infoColorPressed: palette.info,
      infoColorSuppl: palette.info,
      successColor: palette.success,
      successColorHover: palette.success,
      successColorPressed: palette.success,
      successColorSuppl: palette.success,
      warningColor: palette.warning,
      warningColorHover: palette.warning,
      warningColorPressed: palette.warning,
      warningColorSuppl: palette.warning,
      errorColor: palette.error,
      errorColorHover: palette.error,
      errorColorPressed: palette.error,
      errorColorSuppl: palette.error,
      textColorBase: palette.text1,
      textColor1: palette.text1,
      textColor2: palette.text2,
      textColor3: palette.text3,
      textColorDisabled: palette.disabled,
      bodyColor: palette.body,
      cardColor: palette.card,
      modalColor: palette.card,
      popoverColor: palette.popover,
      tableColor: palette.card,
      tableHeaderColor: palette.subtle,
      tabColor: palette.subtle,
      borderColor: palette.border,
      dividerColor: palette.borderSubtle,
      hoverColor: palette.hover,
      iconColor: palette.text3,
      buttonColor2: palette.hover,
      buttonColor2Hover: palette.hover,
      buttonColor2Pressed: palette.border,
    },
    Button: {
      // filled primary uses the (darker) solid blue with white text in both
      // themes; text/link primary keeps the accessible accent
      colorPrimary: palette.solid,
      colorHoverPrimary: palette.solidHover,
      colorPressedPrimary: palette.solidHover,
      colorFocusPrimary: palette.solidHover,
      borderPrimary: `1px solid ${palette.solid}`,
      borderHoverPrimary: `1px solid ${palette.solidHover}`,
      borderPressedPrimary: `1px solid ${palette.solidHover}`,
      borderFocusPrimary: `1px solid ${palette.solidHover}`,
      textColorPrimary: '#FFFFFF',
      textColorHoverPrimary: '#FFFFFF',
      textColorPressedPrimary: '#FFFFFF',
      textColorFocusPrimary: '#FFFFFF',
      fontWeight: '500',
      heightMedium: '36px',
      heightSmall: '32px',
    },
    Card: {
      color: palette.card,
      colorModal: palette.card,
      borderColor: palette.border,
      borderRadius: '12px',
    },
    Dialog: {
      borderRadius: '14px',
    },
    Modal: {
      borderRadius: '14px',
    },
    DataTable: {
      thColor: palette.subtle,
      thColorHover: palette.subtle,
      thTextColor: palette.text2,
      thFontWeight: '600',
      tdColor: palette.card,
      tdColorHover: palette.subtle,
      tdColorStriped: palette.subtle,
      borderColor: palette.borderSubtle,
      fontSizeMedium: '13px',
      fontSizeSmall: '13px',
      borderRadius: '12px',
    },
    Input: {
      borderRadius: '8px',
      heightMedium: '36px',
    },
    Select: {
      peers: {
        InternalSelection: { borderRadius: '8px', heightMedium: '36px' },
      },
    },
    Tag: {
      borderRadius: '6px',
    },
    Tabs: {
      tabBorderColor: palette.border,
    },
    Carousel: {
      dotColor: `${palette.text3}4D`,
      dotColorActive: palette.text3,
      dotColorFocus: `${palette.text3}80`,
    },
  };
}

export const LIGHT_THEME_OVERRIDES: GlobalThemeOverrides = build({
  primary: '#2952CC',
  primaryHover: '#1D3C99',
  primaryPressed: '#1D3C99',
  solid: '#2952CC',
  solidHover: '#1D3C99',
  info: '#175CD3',
  success: '#067647',
  warning: '#B54708',
  error: '#B42318',
  text1: '#101828',
  text2: '#475467',
  text3: '#667085',
  disabled: '#98A2B3',
  body: '#F5F7FA',
  card: '#FFFFFF',
  popover: '#FFFFFF',
  subtle: '#F9FAFB',
  border: '#E4E7EC',
  borderSubtle: '#EEF0F3',
  hover: 'rgba(16, 24, 40, 0.05)',
});

export const DARK_THEME_OVERRIDES: GlobalThemeOverrides = build({
  primary: '#6E95FF',
  primaryHover: '#8FADFF',
  primaryPressed: '#8FADFF',
  solid: '#3A66F0',
  solidHover: '#2F58D6',
  info: '#84ADFF',
  success: '#47CD89',
  warning: '#FDB022',
  error: '#F97066',
  text1: '#F1F5F9',
  text2: '#A9B4C8',
  text3: '#8D99B0',
  disabled: '#66728B',
  body: '#0B1220',
  card: '#111A2E',
  popover: '#16213A',
  subtle: '#16213A',
  border: '#24314D',
  borderSubtle: '#1C2943',
  hover: 'rgba(255, 255, 255, 0.06)',
});
