import $ from 'jquery';
import {initFomanticApiPatch} from './fomantic/api.js';
import {initAriaCheckboxPatch} from './fomantic/checkbox.js';
import {initAriaDropdownPatch} from './fomantic/dropdown.js';
import {initAriaModalPatch} from './fomantic/modal.js';
import {initFomanticTransition} from './fomantic/transition.js';
import {svg} from '../svg.js';

export const fomanticMobileScreen = window.matchMedia('only screen and (max-width: 767.98px)');

export function initGiteaFomantic() {
  // Silence fomantic's error logging when tabs are used without a target content element
  $.fn.tab.settings.silent = true;
  // Disable the behavior of fomantic to toggle the checkbox when you press enter on a checkbox element.
  $.fn.checkbox.settings.enableEnterKey = false;

  // By default, use "exact match" for full text search
  $.fn.dropdown.settings.fullTextSearch = 'exact';
  // Do not use "cursor: pointer" for dropdown labels
  $.fn.dropdown.settings.className.label += ' gt-cursor-default';
  // Always use Gitea's SVG icons
  $.fn.dropdown.settings.templates.label = function(_value, text, preserveHTML, className) {
    const escape = $.fn.dropdown.settings.templates.escape;
    return escape(text, preserveHTML) + svg('octicon-x', 16, `${className.delete} icon`);
  };

  initFomanticTransition();
  initFomanticApiPatch();

  // Use the patches to improve accessibility, these patches are designed to be as independent as possible, make it easy to modify or remove in the future.
  initAriaCheckboxPatch();
  initAriaDropdownPatch();
  initAriaModalPatch();
}
