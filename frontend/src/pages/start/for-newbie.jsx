import cn from 'classnames'
import { IoArrowForwardSharp, IoCheckmarkCircleSharp } from 'react-icons/io5'
import { Button } from '@/components/ui/button'
import { useStartStore } from '../../store/useStartStore'
import { useGetTemplates, useInitTemplate } from '../../apis/template'
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { PiSpinner, PiSpinnerGap } from 'react-icons/pi'

export function ForNewbie() {
    const { type, templateType, setStep, setTemplateType } = useStartStore()
    const navigate = useNavigate()
    const { data: templates } = useGetTemplates()
    const { trigger: initTemplate, loading: initTemplateLoading, error: initTemplateError } = useInitTemplate()
    useEffect(() => {
        if (templates?.length) {
            setTemplateType(templates[0].type)
        }
    }, [templates])
    return (
        <>
            <div className='mt-8 text-xl'>选择模板快速开始</div>
            <div className='opacity-70 text-sm mt-2'>不会选择？随便选一个就行。</div>

            <div className='mt-8'>
                {templates?.map(i => (
                    <div className={cn({
                        'p-4 bg-white cursor-pointer relative first:rounded-tl-lg first:rounded-tr-lg last:rounded-bl-lg last:rounded-br-lg border-b-1 border-b-stone-100 ': true,
                        'outline outline-2 outline-primary rounded-lg z-1000': templateType === i.type
                    })} onClick={() => setTemplateType(i.type)}>
                        <div>{i.name}</div>
                        <div className='text-sm mt-1 opacity-70'>{i.description}</div>
                        {templateType === i.type && <IoCheckmarkCircleSharp className='text-primary absolute right-3 top-3 text-2xl' />}
                    </div>
                ))}
            </div>
            <div className='mt-8 flex justify-between'>
                <Button variant='outline' className='cursor-pointer' onClick={() => setStep('select-user-type')}>返回</Button>
                <Button disabled={initTemplateLoading} className='cursor-pointer' onClick={async() => {
                    await initTemplate({ type: templateType })
                    navigate('/')
                }}>部署 {initTemplateLoading ? <PiSpinnerGap className='animate-spin' /> : <IoArrowForwardSharp />}</Button>
            </div>
            {initTemplateError && <div className='bg-red-800 text-white p-4 mt-8 rounded-lg'>
                {initTemplateError?.message}
            </div>}
        </>
    )
}