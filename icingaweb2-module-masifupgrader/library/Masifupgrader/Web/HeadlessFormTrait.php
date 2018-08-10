<?php

namespace Icinga\Module\Masifupgrader\Web;

use Icinga\Web\Form;
use Zend_Form;
use Zend_Form_Element;
use Zend_View_Interface;

trait HeadlessFormTrait
{
    /**
     * @var Zend_Form_Element[]
     */
    protected $headlessElements = [];

    protected function addHeadlessElement($element, ...$args)
    {
        /** @var Form $that */
        $that = $this;

        if (!($element instanceof Zend_Form_Element)) {
            $element = $that->createElement($element, ...$args);
        }

        $element->form = $this->getId();
        $element->setDecorators([$element->getDecorators()['Zend_Form_Decorator_ViewHelper']]);

        $this->headlessElements[$element->getName()] = $element;

        return $this->addElement($element);
    }

    protected function renderHeadless(Zend_View_Interface $view = null)
    {
        /** @var Form $that */
        $that = $this;

        $that->setAttrib('style', 'display: none;');

        $that->create();

        $element = $that->getElement('btn_submit');
        if ($element !== null) {
            $element->form = $this->getId();
            $element->setDecorators([$element->getDecorators()['Zend_Form_Decorator_ViewHelper']]);

            $this->headlessElements[$element->getName()] = $element;
        }

        foreach ($this->headlessElements as $headlessElement => $_) {
            $that->removeElement($headlessElement);
        }

        $result = Zend_Form::render($view);

        foreach ($this->headlessElements as $headlessElement) {
            $that->addElement($headlessElement);
        }

        return $result;
    }
}
